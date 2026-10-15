package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/rossigee/provider-minio/apis"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakePolicyAdmin is a scriptable minioutil.PolicyAdmin. It records every call
// so tests can assert the exact MinIO mutations the controller performed.
type fakePolicyAdmin struct {
	mu sync.Mutex

	policies map[string]json.RawMessage

	listErr   error
	addErr    error
	removeErr error

	added   []string
	removed []string
}

func newFakePolicyAdmin(policies map[string]json.RawMessage) *fakePolicyAdmin {
	if policies == nil {
		policies = map[string]json.RawMessage{}
	}
	return &fakePolicyAdmin{policies: policies}
}

func (f *fakePolicyAdmin) ListCannedPolicies(_ context.Context) (map[string]json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make(map[string]json.RawMessage, len(f.policies))
	for k, v := range f.policies {
		out[k] = v
	}
	return out, nil
}

func (f *fakePolicyAdmin) AddCannedPolicy(_ context.Context, name string, policy []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, name)
	f.policies[name] = json.RawMessage(policy)
	return nil
}

func (f *fakePolicyAdmin) RemoveCannedPolicy(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removed = append(f.removed, name)
	delete(f.policies, name)
	return nil
}

func testPolicy(name, allowBucket, rawPolicy string) *miniov1beta1.Policy {
	return &miniov1beta1.Policy{
		// A UID is required: the ProviderConfigUsage that Connect records is
		// named after the managed resource UID, and a real object always has one.
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name)},
		Spec: miniov1beta1.PolicySpec{
			ForProvider: miniov1beta1.PolicyParameters{
				AllowBucket: allowBucket,
				RawPolicy:   rawPolicy,
			},
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: "ProviderConfig",
					Name: "provider-config",
				},
			},
		},
	}
}

func TestPolicyClientCreate(t *testing.T) {
	t.Run("AddsAllowBucketPolicy", func(t *testing.T) {
		fake := newFakePolicyAdmin(nil)
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Create(context.Background(), testPolicy("p1", "mybucket", ""))
		require.NoError(t, err)
		assert.Equal(t, []string{"p1"}, fake.added)
		assert.Contains(t, string(fake.policies["p1"]), "mybucket")
	})

	t.Run("AddsRawPolicy", func(t *testing.T) {
		fake := newFakePolicyAdmin(nil)
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Create(context.Background(), testPolicy("p1", "", `{"Version":"2012-10-17"}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"p1"}, fake.added)
		assert.JSONEq(t, `{"Version":"2012-10-17"}`, string(fake.policies["p1"]))
	})

	t.Run("RefusesWhenPolicyAlreadyExists", func(t *testing.T) {
		fake := newFakePolicyAdmin(map[string]json.RawMessage{
			"p1": json.RawMessage(`{"Statement":[]}`),
		})
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Create(context.Background(), testPolicy("p1", "mybucket", ""))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy already exists")
		assert.Empty(t, fake.added, "must not call AddCannedPolicy when the policy exists")
	})

	t.Run("RejectsWhenNoPolicySpecified", func(t *testing.T) {
		fake := newFakePolicyAdmin(nil)
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Create(context.Background(), testPolicy("p1", "", ""))
		require.Error(t, err)
		assert.Empty(t, fake.added)
	})

	t.Run("PropagatesListError", func(t *testing.T) {
		fake := newFakePolicyAdmin(nil)
		fake.listErr = fmt.Errorf("boom")
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Create(context.Background(), testPolicy("p1", "b", ""))
		require.Error(t, err)
	})

	t.Run("RejectsWrongResourceType", func(t *testing.T) {
		p := &policyClient{ma: newFakePolicyAdmin(nil), recorder: event.NewNopRecorder()}
		_, err := p.Create(context.Background(), &miniov1beta1.Bucket{})
		require.ErrorIs(t, err, errNotPolicy)
	})
}

func TestPolicyClientObserve(t *testing.T) {
	t.Run("FindsExistingPolicy", func(t *testing.T) {
		fake := newFakePolicyAdmin(map[string]json.RawMessage{
			"p1": json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
		})
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		got, err := p.Observe(context.Background(), testPolicy("p1", "", `{"Version":"2012-10-17","Statement":[]}`))
		require.NoError(t, err)
		assert.True(t, got.ResourceExists)
	})

	t.Run("ReportsMissingWhenNotFoundAndNotClaimed", func(t *testing.T) {
		p := &policyClient{ma: newFakePolicyAdmin(nil), recorder: event.NewNopRecorder()}

		got, err := p.Observe(context.Background(), testPolicy("p1", "b", ""))
		require.NoError(t, err)
		assert.False(t, got.ResourceExists)
	})

	t.Run("TreatsMissingButClaimedAsGone", func(t *testing.T) {
		pol := testPolicy("p1", "b", "")
		pol.SetAnnotations(map[string]string{PolicyCreatedAnnotationKey: "true"})

		p := &policyClient{ma: newFakePolicyAdmin(nil), recorder: event.NewNopRecorder()}
		got, err := p.Observe(context.Background(), pol)
		require.NoError(t, err)
		assert.False(t, got.ResourceExists)
	})
}

func TestPolicyClientDelete(t *testing.T) {
	t.Run("RemovesPolicy", func(t *testing.T) {
		fake := newFakePolicyAdmin(map[string]json.RawMessage{
			"p1": json.RawMessage(`{"Statement":[]}`),
		})
		p := &policyClient{ma: fake, recorder: event.NewNopRecorder()}

		_, err := p.Delete(context.Background(), testPolicy("p1", "b", ""))
		require.NoError(t, err)
		assert.Equal(t, []string{"p1"}, fake.removed)
		assert.NotContains(t, fake.policies, "p1")
	})
}

// TestConnectorConnect covers connector.go, which had no test coverage at all.
// It asserts that Connect resolves the ProviderConfig and hands the resulting
// client to the reconciler, and that a missing ProviderConfig surfaces as an
// error rather than a nil dereference.
func TestConnectorConnect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))

	pc := &providerv1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "provider-config"},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc).Build()

	t.Run("ResolvesProviderConfig", func(t *testing.T) {
		fakeAdmin := newFakePolicyAdmin(nil)
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, cfg *providerv1beta1.ProviderConfig) (minioutil.PolicyAdmin, error) {
				assert.Equal(t, "provider-config", cfg.GetName())
				return fakeAdmin, nil
			},
		}

		got, err := c.Connect(context.Background(), testPolicy("p1", "b", ""))
		require.NoError(t, err)

		uc, ok := got.(*policyClient)
		require.True(t, ok, "Connect must return a *policyClient")
		assert.Same(t, fakeAdmin, uc.ma, "the injected admin client must reach the external client")
	})

	t.Run("FailsWhenProviderConfigMissing", func(t *testing.T) {
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, _ *providerv1beta1.ProviderConfig) (minioutil.PolicyAdmin, error) {
				t.Fatal("newAdmin must not be called when the ProviderConfig is missing")
				return nil, nil
			},
		}

		pol := testPolicy("p1", "b", "")
		pol.Spec.ProviderConfigReference.Name = "does-not-exist"

		_, err := c.Connect(context.Background(), pol)
		require.Error(t, err)
	})

	t.Run("FailsForWrongResourceType", func(t *testing.T) {
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
		}
		wrong := &miniov1beta1.Bucket{
			ObjectMeta: metav1.ObjectMeta{Name: "b1", UID: types.UID("uid-b1")},
			Spec: miniov1beta1.BucketSpec{ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Kind: "ProviderConfig", Name: "provider-config"},
			}},
		}
		_, err := c.Connect(context.Background(), wrong)
		require.ErrorIs(t, err, errNotPolicy)
	})
}

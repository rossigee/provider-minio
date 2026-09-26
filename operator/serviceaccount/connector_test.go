package serviceaccount

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	"github.com/rossigee/provider-minio/apis"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakeSAAdmin is a scriptable minioutil.ServiceAccountAdmin.
type fakeSAAdmin struct {
	mu sync.Mutex

	accounts map[string]madmin.InfoServiceAccountResp
	users    map[string]madmin.UserInfo

	infoErr    error
	addErr     error
	updateErr  error
	deleteErr  error
	attachErr  error
	detachErr  error
	getUserErr error

	added    []string
	deleted  []string
	attached []string
	detached []string
}

func newFakeSAAdmin() *fakeSAAdmin {
	return &fakeSAAdmin{
		accounts: map[string]madmin.InfoServiceAccountResp{},
		users:    map[string]madmin.UserInfo{},
	}
}

func (f *fakeSAAdmin) AddServiceAccount(_ context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return madmin.Credentials{}, f.addErr
	}
	accessKey := opts.AccessKey
	if accessKey == "" {
		accessKey = "generated-key"
	}
	f.added = append(f.added, accessKey)
	f.accounts[accessKey] = madmin.InfoServiceAccountResp{AccountStatus: "on", ParentUser: opts.TargetUser}
	return madmin.Credentials{AccessKey: accessKey, SecretKey: opts.SecretKey}, nil
}

func (f *fakeSAAdmin) InfoServiceAccount(_ context.Context, accessKey string) (madmin.InfoServiceAccountResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.infoErr != nil {
		return madmin.InfoServiceAccountResp{}, f.infoErr
	}
	info, ok := f.accounts[accessKey]
	if !ok {
		// The controller distinguishes not-found by matching this substring.
		return madmin.InfoServiceAccountResp{}, fmt.Errorf("service account does not exist")
	}
	return info, nil
}

func (f *fakeSAAdmin) GetUserInfo(_ context.Context, name string) (madmin.UserInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getUserErr != nil {
		return madmin.UserInfo{}, f.getUserErr
	}
	info, ok := f.users[name]
	if !ok {
		return madmin.UserInfo{}, fmt.Errorf("user does not exist")
	}
	return info, nil
}

func (f *fakeSAAdmin) UpdateServiceAccount(_ context.Context, accessKey string, _ madmin.UpdateServiceAccountReq) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateErr
}

func (f *fakeSAAdmin) DeleteServiceAccount(_ context.Context, accessKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, accessKey)
	delete(f.accounts, accessKey)
	return nil
}

func (f *fakeSAAdmin) AttachPolicy(_ context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attachErr != nil {
		return madmin.PolicyAssociationResp{}, f.attachErr
	}
	f.attached = append(f.attached, r.Policies...)
	return madmin.PolicyAssociationResp{}, nil
}

func (f *fakeSAAdmin) DetachPolicy(_ context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.detachErr != nil {
		return madmin.PolicyAssociationResp{}, f.detachErr
	}
	f.detached = append(f.detached, r.Policies...)
	return madmin.PolicyAssociationResp{}, nil
}

func testSA(accessKey string, mutate ...func(*miniov1beta1.ServiceAccount)) *miniov1beta1.ServiceAccount {
	sa := &miniov1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa",
			Namespace: "default",
			UID:       types.UID("uid-sa"),
		},
		Spec: miniov1beta1.ServiceAccountSpec{
			ForProvider: miniov1beta1.ServiceAccountParameters{
				AccessKey: accessKey,
				SecretKey: "supersecret",
			},
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: "ProviderConfig",
					Name: "provider-config",
				},
				WriteConnectionSecretToReference: &xpv1.LocalSecretReference{
					Name: "sa-conn",
				},
			},
		},
	}
	for _, m := range mutate {
		m(sa)
	}
	return sa
}

func newTestSAClient(t *testing.T, ma minioutil.ServiceAccountAdmin) *serviceAccountClient {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return &serviceAccountClient{
		ma:       ma,
		kube:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		recorder: event.NewNopRecorder(),
	}
}

func TestServiceAccountClientCreate(t *testing.T) {
	t.Run("AddsServiceAccount", func(t *testing.T) {
		ma := newFakeSAAdmin()
		sa := newTestSAClient(t, ma)

		_, err := sa.Create(context.Background(), testSA("AKIA"))
		require.NoError(t, err)
		assert.Equal(t, []string{"AKIA"}, ma.added)
	})

	t.Run("RejectsBothPolicyAndPolicies", func(t *testing.T) {
		ma := newFakeSAAdmin()
		sa := newTestSAClient(t, ma)

		_, err := sa.Create(context.Background(), testSA("AKIA", func(s *miniov1beta1.ServiceAccount) {
			s.Spec.ForProvider.Policy = "readonly"
			s.Spec.ForProvider.Policies = []string{"writeonly"}
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only one of policy or policies")
		assert.Empty(t, ma.added)
	})

	t.Run("RejectsWrongResourceType", func(t *testing.T) {
		sa := newTestSAClient(t, newFakeSAAdmin())
		_, err := sa.Create(context.Background(), &miniov1beta1.Bucket{})
		require.ErrorIs(t, err, errNotServiceAccount)
	})
}

func TestServiceAccountClientObserve(t *testing.T) {
	t.Run("FindsExistingServiceAccount", func(t *testing.T) {
		ma := newFakeSAAdmin()
		ma.accounts["AKIA"] = madmin.InfoServiceAccountResp{AccountStatus: "on", ParentUser: "alice"}
		sa := newTestSAClient(t, ma)

		got, err := sa.Observe(context.Background(), testSA("AKIA"))
		require.NoError(t, err)
		assert.True(t, got.ResourceExists)
	})

	t.Run("ReportsMissingWhenAbsent", func(t *testing.T) {
		sa := newTestSAClient(t, newFakeSAAdmin())
		got, err := sa.Observe(context.Background(), testSA("AKIA"))
		require.NoError(t, err)
		assert.False(t, got.ResourceExists)
	})

	t.Run("PropagatesTransientError", func(t *testing.T) {
		// A non not-found error must be returned so the reconciler requeues,
		// rather than being mistaken for "does not exist".
		ma := newFakeSAAdmin()
		ma.infoErr = fmt.Errorf("connection refused")
		sa := newTestSAClient(t, ma)

		_, err := sa.Observe(context.Background(), testSA("AKIA"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})
}

func TestServiceAccountClientDelete(t *testing.T) {
	// Delete resolves the MinIO access key from the external-name annotation,
	// not from spec.forProvider, so the fixtures must set that annotation.
	claimed := func(sa *miniov1beta1.ServiceAccount) {
		sa.SetAnnotations(map[string]string{meta.AnnotationKeyExternalName: "AKIA"})
	}

	t.Run("RemovesServiceAccount", func(t *testing.T) {
		ma := newFakeSAAdmin()
		ma.accounts["AKIA"] = madmin.InfoServiceAccountResp{AccountStatus: "on"}
		sa := newTestSAClient(t, ma)

		_, err := sa.Delete(context.Background(), testSA("AKIA", claimed))
		require.NoError(t, err)
		assert.Equal(t, []string{"AKIA"}, ma.deleted)
	})

	t.Run("NoOpWhenNeverCreated", func(t *testing.T) {
		ma := newFakeSAAdmin()
		sa := newTestSAClient(t, ma)

		// No external-name annotation means the resource was never created, so
		// deletion must succeed without calling MinIO.
		_, err := sa.Delete(context.Background(), testSA("AKIA"))
		require.NoError(t, err)
		assert.Empty(t, ma.deleted)
	})

	t.Run("NoOpWhenAlreadyGone", func(t *testing.T) {
		ma := newFakeSAAdmin()
		sa := newTestSAClient(t, ma)

		_, err := sa.Delete(context.Background(), testSA("AKIA", claimed))
		require.NoError(t, err)
		assert.Empty(t, ma.deleted, "must not call MinIO when the service account is already absent")
	})
}

// TestSAConnectorConnect covers serviceaccount/connector.go, previously untested.
func TestSAConnectorConnect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))

	pc := &providerv1beta1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "provider-config"}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc).Build()

	t.Run("ResolvesProviderConfig", func(t *testing.T) {
		ma := newFakeSAAdmin()
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, cfg *providerv1beta1.ProviderConfig) (minioutil.ServiceAccountAdmin, error) {
				assert.Equal(t, "provider-config", cfg.GetName())
				return ma, nil
			},
		}

		got, err := c.Connect(context.Background(), testSA("AKIA"))
		require.NoError(t, err)
		uc, ok := got.(*serviceAccountClient)
		require.True(t, ok)
		assert.Same(t, ma, uc.ma)
	})
}

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

		// The account exists and its connection secret is present, so Observe
		// must not report the credentials as lost.
		require.NoError(t, sa.kube.Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sa-conn", Namespace: "default"},
			Data:       map[string][]byte{AccessKeyName: []byte("AKIA")},
		}))

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

// TestPoliciesEqual pins the fix for an infinite update loop: MinIO
// re-serialises the policy it stores, so a raw string compare never matched and
// the controller called UpdateServiceAccount on every reconcile, leaving the
// resource stuck at Updating forever.
func TestPoliciesEqual(t *testing.T) {
	cases := map[string]struct {
		a, b string
		want bool
	}{
		"Identical":            {`{"a":1}`, `{"a":1}`, true},
		"WhitespaceDiffers":    {"{\n  \"a\": 1\n}", `{"a":1}`, true},
		"KeyOrderDiffers":      {`{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		"NestedOrderDiffers":   {`{"s":[{"x":1,"y":2}]}`, `{"s":[{"y":2,"x":1}]}`, true},
		"DifferentValue":       {`{"a":1}`, `{"a":2}`, false},
		"ExtraKey":             {`{"a":1}`, `{"a":1,"b":2}`, false},
		"MinioNormalisedEmpty": {`{"Version":"2012-10-17","Statement":[]}`, `{"Statement":[],"Version":"2012-10-17"}`, true},
		// The real case found by the e2e suite: MinIO sorts Action.
		"MinioSortsActionArray": {
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject","s3:ListBucket"],"Resource":["arn:aws:s3:::b","arn:aws:s3:::b/*"]}]}`,
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket","s3:PutObject"],"Resource":["arn:aws:s3:::b","arn:aws:s3:::b/*"]}]}`,
			true,
		},
		"MinioSortsResourceArray": {
			`{"Statement":[{"Resource":["z","a"]}]}`,
			`{"Statement":[{"Resource":["a","z"]}]}`,
			true,
		},
		"UnparseableSpec":   {`not json`, `not json`, true},
		"UnparseableRemote": {`{"a":1}`, `not json`, false},
		"EmptyRemote":       {`{"a":1}`, ``, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, policiesEqual(tc.a, tc.b))
		})
	}
}

// TestAccountStatusEnabled pins the fix for the second bug: madmin reports
// "enabled" for users but InfoServiceAccountResp reports "on" for service
// accounts, so comparing only against "enabled" left every service account
// reported Disabled.
func TestAccountStatusEnabled(t *testing.T) {
	assert.True(t, accountStatusEnabled("on"), "MinIO reports on for service accounts")
	assert.True(t, accountStatusEnabled("enabled"), "madmin.AccountEnabled")
	assert.True(t, accountStatusEnabled(string(madmin.AccountEnabled)))
	assert.False(t, accountStatusEnabled("off"))
	assert.False(t, accountStatusEnabled("disabled"))
	assert.False(t, accountStatusEnabled(""))
}

// TestIsUpToDateConvergesForEquivalentPolicy is the regression test for the
// loop itself: a spec policy that differs from MinIO's re-serialised copy only
// in formatting must be reported up to date, or the controller never settles.
func TestIsUpToDateConvergesForEquivalentPolicy(t *testing.T) {
	sa := newTestSAClient(t, newFakeSAAdmin())

	specPolicy := `{
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": ["s3:GetObject", "s3:PutObject", "s3:ListBucket"],
          "Resource": ["arn:aws:s3:::test-bucket", "arn:aws:s3:::test-bucket/*"]
        }
      ]
    }`
	// What MinIO actually returned, compact and reordered.
	remotePolicy := `{"Statement":[{"Action":["s3:GetObject","s3:ListBucket","s3:PutObject"],"Effect":"Allow","Resource":["arn:aws:s3:::test-bucket","arn:aws:s3:::test-bucket/*"]}],"Version":"2012-10-17"}`

	saRef := testSA("AKIA", func(s *miniov1beta1.ServiceAccount) {
		s.Spec.ForProvider.Policy = specPolicy
	})

	info := madmin.InfoServiceAccountResp{
		AccountStatus: "on",
		Policy:        remotePolicy,
	}

	assert.True(t, sa.isUpToDate(saRef, info),
		"an equivalent policy must be considered up to date, or the controller loops forever")
}

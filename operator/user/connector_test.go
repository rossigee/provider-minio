package user

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
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

// fakeUserAdmin is a scriptable minioutil.UserAdmin that records every call, so
// tests can assert the exact MinIO mutations the controller performed.
type fakeUserAdmin struct {
	mu sync.Mutex

	users map[string]madmin.UserInfo

	listErr    error
	addErr     error
	removeErr  error
	attachErr  error
	detachErr  error
	setUserErr error
	getInfoErr error

	added    []string
	removed  []string
	attached []string
	detached []string
}

func newFakeUserAdmin(users map[string]madmin.UserInfo) *fakeUserAdmin {
	if users == nil {
		users = map[string]madmin.UserInfo{}
	}
	return &fakeUserAdmin{users: users}
}

func (f *fakeUserAdmin) AddUser(_ context.Context, accessKey, secretKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, accessKey)
	f.users[accessKey] = madmin.UserInfo{Status: madmin.AccountEnabled, SecretKey: secretKey}
	return nil
}

func (f *fakeUserAdmin) RemoveUser(_ context.Context, accessKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removed = append(f.removed, accessKey)
	delete(f.users, accessKey)
	return nil
}

func (f *fakeUserAdmin) ListUsers(_ context.Context) (map[string]madmin.UserInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make(map[string]madmin.UserInfo, len(f.users))
	for k, v := range f.users {
		out[k] = v
	}
	return out, nil
}

func (f *fakeUserAdmin) GetUserInfo(_ context.Context, name string) (madmin.UserInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getInfoErr != nil {
		return madmin.UserInfo{}, f.getInfoErr
	}
	info, ok := f.users[name]
	if !ok {
		return madmin.UserInfo{}, fmt.Errorf("user does not exist")
	}
	return info, nil
}

func (f *fakeUserAdmin) SetUser(_ context.Context, accessKey, secretKey string, status madmin.AccountStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setUserErr != nil {
		return f.setUserErr
	}
	f.users[accessKey] = madmin.UserInfo{Status: status, SecretKey: secretKey}
	return nil
}

func (f *fakeUserAdmin) AttachPolicy(_ context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attachErr != nil {
		return madmin.PolicyAssociationResp{}, f.attachErr
	}
	f.attached = append(f.attached, r.Policies...)
	return madmin.PolicyAssociationResp{}, nil
}

func (f *fakeUserAdmin) DetachPolicy(_ context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.detachErr != nil {
		return madmin.PolicyAssociationResp{}, f.detachErr
	}
	f.detached = append(f.detached, r.Policies...)
	return madmin.PolicyAssociationResp{}, nil
}

// fakeUserS3 stands in for the credential-verification client.
type fakeUserS3 struct {
	calls int
	err   error
}

func (f *fakeUserS3) ListBuckets(_ context.Context) ([]minio.BucketInfo, error) {
	f.calls++
	return nil, f.err
}

func testUser(name string, policies ...string) *miniov1beta1.User {
	return &miniov1beta1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID("uid-" + name),
		},
		Spec: miniov1beta1.UserSpec{
			ForProvider: miniov1beta1.UserParameters{Policies: policies},
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: "ProviderConfig",
					Name: "provider-config",
				},
				WriteConnectionSecretToReference: &xpv1.LocalSecretReference{Name: name + "-conn"},
			},
		},
	}
}

func newTestUserClient(t *testing.T, ma minioutil.UserAdmin, objs ...client.Object) (*userClient, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &userClient{ma: ma, kube: kube, recorder: event.NewNopRecorder()}, kube
}

func TestUserClientCreate(t *testing.T) {
	t.Run("AddsUser", func(t *testing.T) {
		ma := newFakeUserAdmin(nil)
		uc, _ := newTestUserClient(t, ma)

		creds, err := uc.Create(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.Equal(t, []string{"alice"}, ma.added)
		assert.NotEmpty(t, creds.ConnectionDetails[SecretKeyName], "a generated secret key must be returned")
		assert.Equal(t, []byte("alice"), creds.ConnectionDetails[AccessKeyName])
	})

	t.Run("RefusesWhenUserExists", func(t *testing.T) {
		ma := newFakeUserAdmin(map[string]madmin.UserInfo{"alice": {Status: madmin.AccountEnabled}})
		uc, _ := newTestUserClient(t, ma)

		_, err := uc.Create(context.Background(), testUser("alice"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user already exists")
		assert.Empty(t, ma.added, "must not call AddUser when the user already exists")
	})

	t.Run("AttachesPolicies", func(t *testing.T) {
		ma := newFakeUserAdmin(nil)
		uc, _ := newTestUserClient(t, ma)

		_, err := uc.Create(context.Background(), testUser("alice", "readonly", "writeonly"))
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"readonly", "writeonly"}, ma.attached)
	})

	t.Run("SetsLockAnnotation", func(t *testing.T) {
		ma := newFakeUserAdmin(nil)
		uc, _ := newTestUserClient(t, ma)

		u := testUser("alice")
		require.Nil(t, u.GetAnnotations(), "fixture must have no annotations to exercise the nil-map path")

		_, err := uc.Create(context.Background(), u)
		require.NoError(t, err)
		assert.Equal(t, "true", u.GetAnnotations()[UserCreatedAnnotationKey])
	})

	t.Run("RejectsWrongResourceType", func(t *testing.T) {
		uc, _ := newTestUserClient(t, newFakeUserAdmin(nil))
		_, err := uc.Create(context.Background(), &miniov1beta1.Bucket{})
		require.ErrorIs(t, err, errNotUser)
	})
}

func TestUserClientObserve(t *testing.T) {
	t.Run("FindsExistingUser", func(t *testing.T) {
		ma := newFakeUserAdmin(map[string]madmin.UserInfo{"alice": {Status: madmin.AccountEnabled}})
		uc, _ := newTestUserClient(t, ma)

		got, err := uc.Observe(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.True(t, got.ResourceExists)
	})

	t.Run("ReportsMissingWhenAbsent", func(t *testing.T) {
		uc, _ := newTestUserClient(t, newFakeUserAdmin(nil))
		got, err := uc.Observe(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.False(t, got.ResourceExists)
	})
}

func TestUserClientDelete(t *testing.T) {
	t.Run("RemovesUser", func(t *testing.T) {
		ma := newFakeUserAdmin(map[string]madmin.UserInfo{"alice": {Status: madmin.AccountEnabled}})
		uc, _ := newTestUserClient(t, ma)

		_, err := uc.Delete(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.Equal(t, []string{"alice"}, ma.removed)
		assert.NotContains(t, ma.users, "alice")
	})

	t.Run("PropagatesRemoveError", func(t *testing.T) {
		ma := newFakeUserAdmin(nil)
		ma.removeErr = fmt.Errorf("admin unavailable")
		uc, _ := newTestUserClient(t, ma)

		_, err := uc.Delete(context.Background(), testUser("alice"))
		require.Error(t, err)
	})
}

// TestUserCredentialClientIsInjectable covers the S3 client that Observe builds
// from the connection secret purely to confirm the credentials work. It used to
// be constructed inline, which made the branch untestable.
func TestUserCredentialClientIsInjectable(t *testing.T) {
	t.Run("UsesInjectedClient", func(t *testing.T) {
		s3 := &fakeUserS3{}
		// The user must already exist in MinIO, otherwise Observe returns early
		// with ResourceExists=false and never reaches the credential check.
		uc, kube := newTestUserClient(t, newFakeUserAdmin(map[string]madmin.UserInfo{
			"alice": {Status: madmin.AccountEnabled},
		}))
		uc.newCredentialClient = func(_ context.Context, accessKey, secretKey string) (minioutil.UserS3, error) {
			assert.Equal(t, "AKIA", accessKey)
			assert.Equal(t, "sekrit", secretKey)
			return s3, nil
		}

		conn := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "alice-conn", Namespace: "default"},
			Data:       map[string][]byte{AccessKeyName: []byte("AKIA"), SecretKeyName: []byte("sekrit")},
		}
		require.NoError(t, kube.Create(context.Background(), conn))

		got, err := uc.Observe(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.Equal(t, 1, s3.calls, "the credential client must actually be exercised")
		assert.True(t, got.ResourceExists)
		assert.True(t, got.ResourceUpToDate)
	})

	t.Run("ToleratesAccessDenied", func(t *testing.T) {
		// AccessDenied proves the credentials reached MinIO, so the user is
		// considered working and must not be reported as out of date.
		s3 := &fakeUserS3{err: fmt.Errorf("Access Denied.")}
		uc, kube := newTestUserClient(t, newFakeUserAdmin(map[string]madmin.UserInfo{"alice": {Status: madmin.AccountEnabled}}))
		uc.newCredentialClient = func(_ context.Context, _, _ string) (minioutil.UserS3, error) { return s3, nil }

		conn := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "alice-conn", Namespace: "default"},
			Data:       map[string][]byte{AccessKeyName: []byte("AKIA"), SecretKeyName: []byte("sekrit")},
		}
		require.NoError(t, kube.Create(context.Background(), conn))

		got, err := uc.Observe(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.True(t, got.ResourceUpToDate)
	})

	t.Run("ReportsOutOfDateOnOtherError", func(t *testing.T) {
		s3 := &fakeUserS3{err: fmt.Errorf("connection refused")}
		uc, kube := newTestUserClient(t, newFakeUserAdmin(map[string]madmin.UserInfo{"alice": {Status: madmin.AccountEnabled}}))
		uc.newCredentialClient = func(_ context.Context, _, _ string) (minioutil.UserS3, error) { return s3, nil }

		conn := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "alice-conn", Namespace: "default"},
			Data:       map[string][]byte{AccessKeyName: []byte("AKIA"), SecretKeyName: []byte("sekrit")},
		}
		require.NoError(t, kube.Create(context.Background(), conn))

		got, err := uc.Observe(context.Background(), testUser("alice"))
		require.NoError(t, err)
		assert.False(t, got.ResourceUpToDate)
	})
}

// TestConnectorConnect covers user/connector.go, previously untested.
func TestConnectorConnect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))

	pc := &providerv1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "provider-config"},
		Spec:       providerv1beta1.ProviderConfigSpec{MinioURL: "http://minio.example:9000/"},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc).Build()

	t.Run("ResolvesProviderConfig", func(t *testing.T) {
		ma := newFakeUserAdmin(nil)
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, cfg *providerv1beta1.ProviderConfig) (minioutil.UserAdmin, error) {
				assert.Equal(t, "provider-config", cfg.GetName())
				return ma, nil
			},
		}

		got, err := c.Connect(context.Background(), testUser("alice"))
		require.NoError(t, err)

		uc, ok := got.(*userClient)
		require.True(t, ok)
		assert.Same(t, ma, uc.ma)
		assert.Equal(t, "minio.example:9000", uc.url.Host)
	})

	t.Run("FailsOnUnparseableMinioURL", func(t *testing.T) {
		bad := &providerv1beta1.ProviderConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-url"},
			Spec:       providerv1beta1.ProviderConfigSpec{MinioURL: "://not a url"},
		}
		badKube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(bad).Build()

		c := &connector{
			kube:  badKube,
			usage: resource.NewProviderConfigUsageTracker(badKube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, _ *providerv1beta1.ProviderConfig) (minioutil.UserAdmin, error) {
				return newFakeUserAdmin(nil), nil
			},
		}

		u := testUser("alice")
		u.Spec.ProviderConfigReference.Name = "bad-url"

		_, err := c.Connect(context.Background(), u)
		require.Error(t, err)
	})

	t.Run("FailsWhenProviderConfigMissing", func(t *testing.T) {
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newAdmin: func(_ context.Context, _ client.Client, _ *providerv1beta1.ProviderConfig) (minioutil.UserAdmin, error) {
				t.Fatal("newAdmin must not be called when the ProviderConfig is missing")
				return nil, nil
			},
		}

		u := testUser("alice")
		u.Spec.ProviderConfigReference.Name = "does-not-exist"

		_, err := c.Connect(context.Background(), u)
		require.Error(t, err)
	})
}

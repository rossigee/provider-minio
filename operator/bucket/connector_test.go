package bucket

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
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

// recordingBucketS3 wraps fakeBucketS3 and records the mutations Create performs.
type recordingBucketS3 struct {
	*fakeBucketS3

	mu          sync.Mutex
	madeBuckets []string
	setPolicy   []string
	setTags     []string
	makeErr     error
}

func newRecordingBucketS3() *recordingBucketS3 {
	return &recordingBucketS3{fakeBucketS3: &fakeBucketS3{exists: false}}
}

func (f *recordingBucketS3) MakeBucket(_ context.Context, name string, _ minio.MakeBucketOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.makeErr != nil {
		return f.makeErr
	}
	f.madeBuckets = append(f.madeBuckets, name)
	return nil
}

func (f *recordingBucketS3) SetBucketPolicy(_ context.Context, name, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setPolicy = append(f.setPolicy, name)
	return nil
}

func (f *recordingBucketS3) SetBucketTagging(_ context.Context, name string, _ *tags.Tags) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setTags = append(f.setTags, name)
	return nil
}

func testBucket(name, bucketName string) *miniov1beta1.Bucket {
	return &miniov1beta1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID("uid-" + name),
		},
		Spec: miniov1beta1.BucketSpec{
			ForProvider: miniov1beta1.BucketParameters{
				BucketName: bucketName,
				Region:     "us-east-1",
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

func TestBucketClientCreate(t *testing.T) {
	t.Run("CreatesBucketAndSetsLock", func(t *testing.T) {
		mc := newRecordingBucketS3()
		b := &bucketClient{mc: mc, recorder: event.NewNopRecorder()}

		bk := testBucket("b1", "my-bucket")
		require.Nil(t, bk.GetAnnotations(), "fixture must have no annotations to exercise the nil-map path")

		_, err := b.Create(context.Background(), bk)
		require.NoError(t, err)
		assert.Equal(t, []string{"my-bucket"}, mc.madeBuckets)
		assert.Equal(t, "claimed", bk.GetAnnotations()[lockAnnotation])
	})

	t.Run("AppliesPolicyAndTags", func(t *testing.T) {
		mc := newRecordingBucketS3()
		b := &bucketClient{mc: mc, recorder: event.NewNopRecorder()}

		policy := `{"Version":"2012-10-17"}`
		bk := testBucket("b1", "my-bucket")
		bk.Spec.ForProvider.Policy = &policy
		bk.Spec.ForProvider.Tags = map[string]string{"env": "test"}

		_, err := b.Create(context.Background(), bk)
		require.NoError(t, err)
		assert.Equal(t, []string{"my-bucket"}, mc.setPolicy)
		assert.Equal(t, []string{"my-bucket"}, mc.setTags)
	})

	t.Run("AdoptsWhenMakeBucketFailsButExists", func(t *testing.T) {
		// A MakeBucket failure followed by a successful BucketExists means we
		// already own the bucket, so Create must succeed rather than error.
		mc := newRecordingBucketS3()
		mc.makeErr = fmt.Errorf("BucketAlreadyOwnedByYou")
		mc.exists = true
		b := &bucketClient{mc: mc, recorder: event.NewNopRecorder()}

		_, err := b.Create(context.Background(), testBucket("b1", "my-bucket"))
		require.NoError(t, err)
	})

	t.Run("ErrorsWhenBucketOwnedByAnother", func(t *testing.T) {
		mc := newRecordingBucketS3()
		mc.makeErr = fmt.Errorf("AccessDenied: bucket belongs to someone else")
		mc.exists = false
		b := &bucketClient{mc: mc, recorder: event.NewNopRecorder()}

		_, err := b.Create(context.Background(), testBucket("b1", "my-bucket"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "belongs to someone else")
	})

	t.Run("RejectsWrongResourceType", func(t *testing.T) {
		mc := newRecordingBucketS3()
		b := &bucketClient{mc: mc, recorder: event.NewNopRecorder()}
		_, err := b.Create(context.Background(), &miniov1beta1.Policy{})
		require.ErrorIs(t, err, errNotBucket)
	})
}

// TestBucketConnectorConnect covers bucket/connector.go, previously untested.
func TestBucketConnectorConnect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))

	pc := &providerv1beta1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "provider-config"}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc).Build()

	t.Run("ResolvesProviderConfig", func(t *testing.T) {
		mc := newRecordingBucketS3()
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newS3: func(_ context.Context, _ client.Client, cfg *providerv1beta1.ProviderConfig) (minioutil.BucketS3, error) {
				assert.Equal(t, "provider-config", cfg.GetName())
				return mc, nil
			},
		}

		got, err := c.Connect(context.Background(), testBucket("b1", "my-bucket"))
		require.NoError(t, err)
		uc, ok := got.(*bucketClient)
		require.True(t, ok)
		assert.Same(t, mc, uc.mc)
	})
}

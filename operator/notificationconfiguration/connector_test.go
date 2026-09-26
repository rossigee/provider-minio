package notificationconfiguration

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7/pkg/notification"
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

const testWebhookARN = "arn:minio:sqs:us-east-1:_:webhook"

// fakeNotificationS3 is a scriptable minioutil.NotificationS3.
type fakeNotificationS3 struct {
	mu sync.Mutex

	config notification.Configuration
	getErr error
	setErr error

	setCalls int
	setWith  notification.Configuration
}

func (f *fakeNotificationS3) GetBucketNotification(_ context.Context, _ string) (notification.Configuration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return notification.Configuration{}, f.getErr
	}
	return f.config, nil
}

func (f *fakeNotificationS3) SetBucketNotification(_ context.Context, _ string, config notification.Configuration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls++
	f.setWith = config
	f.config = config
	return nil
}

func testNC(name string, withWebhook bool) *miniov1beta1.NotificationConfiguration {
	nc := &miniov1beta1.NotificationConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID("uid-" + name),
		},
		Spec: miniov1beta1.NotificationConfigurationSpec{
			ForProvider: miniov1beta1.NotificationConfigurationParameters{
				BucketName: "my-bucket",
			},
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: "ProviderConfig",
					Name: "provider-config",
				},
			},
		},
	}
	if withWebhook {
		nc.Spec.ForProvider.WebhookConfiguration = &miniov1beta1.WebhookConfiguration{
			Endpoint: "https://example.com/hook",
		}
	}
	return nc
}

func newTestNCClient(t *testing.T, mc minioutil.NotificationS3) *notificationClient {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return &notificationClient{
		mc:       mc,
		kube:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		recorder: event.NewNopRecorder(),
	}
}

func TestNotificationClientObserve(t *testing.T) {
	t.Run("FindsExistingWebhookConfig", func(t *testing.T) {
		mc := &fakeNotificationS3{config: notification.Configuration{
			QueueConfigs: []notification.QueueConfig{{Queue: testWebhookARN, Events: []notification.EventType{notification.ObjectCreatedAll}}},
		}}
		nc := newTestNCClient(t, mc)

		got, err := nc.Observe(context.Background(), testNC("nc1", true))
		require.NoError(t, err)
		assert.True(t, got.ResourceExists)
	})

	t.Run("ReportsMissingWhenWebhookAbsent", func(t *testing.T) {
		nc := newTestNCClient(t, &fakeNotificationS3{})

		got, err := nc.Observe(context.Background(), testNC("nc1", true))
		require.NoError(t, err)
		assert.False(t, got.ResourceExists, "a webhook ARN that is not configured must trigger creation")
	})

	t.Run("PropagatesGetError", func(t *testing.T) {
		nc := newTestNCClient(t, &fakeNotificationS3{getErr: fmt.Errorf("bucket not found")})

		_, err := nc.Observe(context.Background(), testNC("nc1", true))
		require.Error(t, err)
	})
}

// TestNCConnectorConnect covers notificationconfiguration/connector.go.
func TestNCConnectorConnect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(scheme))

	pc := &providerv1beta1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "provider-config"}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc).Build()

	t.Run("ResolvesProviderConfig", func(t *testing.T) {
		mc := &fakeNotificationS3{}
		c := &connector{
			kube:  kube,
			usage: resource.NewProviderConfigUsageTracker(kube, &providerv1beta1.ProviderConfigUsage{}),
			newS3: func(_ context.Context, _ client.Client, cfg *providerv1beta1.ProviderConfig) (minioutil.NotificationS3, error) {
				assert.Equal(t, "provider-config", cfg.GetName())
				return mc, nil
			},
		}

		got, err := c.Connect(context.Background(), testNC("nc1", true))
		require.NoError(t, err)
		uc, ok := got.(*notificationClient)
		require.True(t, ok)
		assert.Same(t, mc, uc.mc)
	})
}

package notificationconfiguration

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/notification"

	"github.com/rossigee/provider-minio/apis/minio/v1beta1"
	apisv1alpha1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/internal/clients"
)

const (
	errNotNotificationConfiguration = "managed resource is not a NotificationConfiguration custom resource"
	errTrackPCUsage                 = "cannot track ProviderConfig usage"
	errGetPC                        = "cannot get ProviderConfig"
	errGetCreds                     = "cannot get credentials"
	errNewClient                    = "cannot create new MinIO client"
	errCreateNotification           = "cannot create notification configuration"
	errUpdateNotification           = "cannot update notification configuration"
	errDeleteNotification           = "cannot delete notification configuration"
	errGetNotification              = "cannot get notification configuration"
	errNotificationExists           = "notification configuration already exists"
)

// Setup adds a controller that reconciles NotificationConfiguration managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.NotificationConfigurationGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.NotificationConfiguration{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.NotificationConfigurationGroupVersionKind),
			managed.WithExternalConnecter(&connector{
				kube:         mgr.GetClient(),
				usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
				newServiceFn: clients.NewMinIOClient,
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
			managed.WithPollInterval(o.PollInterval),
		))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(cfg clients.Config) (*madmin.AdminClient, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.NotificationConfiguration)
	if !ok {
		return nil, errors.New(errNotNotificationConfiguration)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cfg, err := clients.GetConfig(ctx, c.kube, pc)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	client, err := c.newServiceFn(*cfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{client: client, cfg: cfg}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client *madmin.AdminClient
	cfg    *clients.Config
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNotificationConfiguration)
	}

	bucketName := cr.Spec.ForProvider.BucketName

	// Create a minio-go client for bucket operations
	minioClient, err := minio.New(c.cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.cfg.AccessKey, c.cfg.SecretKey, ""),
		Secure: c.cfg.UseSSL,
	})
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot create MinIO client")
	}

	// Get current bucket notification configuration
	config, err := minioClient.GetBucketNotification(ctx, bucketName)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetNotification)
	}

	// Check if our webhook configuration exists
	webhookExists := false
	if cr.Spec.ForProvider.WebhookConfiguration != nil {
		webhookID := cr.Spec.ForProvider.WebhookConfiguration.ID
		expectedARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookID)

		for _, webhook := range config.CloudWatchConfigs {
			if webhook.Arn == expectedARN {
				webhookExists = true
				break
			}
		}
	}

	if !webhookExists {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Update status
	cr.Status.AtProvider.BucketName = bucketName
	if cr.Spec.ForProvider.WebhookConfiguration != nil {
		cr.Status.AtProvider.ConfigurationID = cr.Spec.ForProvider.WebhookConfiguration.ID
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNotificationConfiguration)
	}

	webhookConfig := cr.Spec.ForProvider.WebhookConfiguration
	if webhookConfig == nil {
		return managed.ExternalCreation{}, errors.New("webhook configuration is required")
	}

	// First, configure the webhook target in MinIO admin
	webhookID := webhookConfig.ID
	endpoint := webhookConfig.Endpoint

	// Set webhook configuration using SetConfigKV
	configKey := fmt.Sprintf("notify_webhook:%s", webhookID)
	configValue := fmt.Sprintf("endpoint=%s auth_token=", endpoint)

	err := c.client.SetConfigKV(ctx, configKey, configValue)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNotification)
	}

	// Restart MinIO to apply configuration
	err = c.client.ServiceRestart(ctx)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot restart MinIO service")
	}

	// Create a minio-go client for bucket operations
	minioClient, err := minio.New(c.cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.cfg.AccessKey, c.cfg.SecretKey, ""),
		Secure: c.cfg.UseSSL,
	})
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create MinIO client")
	}

	// Configure bucket notification
	bucketNotification := notification.Configuration{}

	// Create webhook configuration
	webhookNotification := notification.CloudWatchConfig{
		Arn: fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookID),
	}

	// Add events
	for _, event := range cr.Spec.ForProvider.Events {
		webhookNotification.Events = append(webhookNotification.Events, notification.EventType(event))
	}

	// Add filter if specified
	if filter := cr.Spec.ForProvider.Filter; filter != nil {
		webhookNotification.Filter = &notification.Filter{
			Key: notification.S3Key{
				FilterRules: []notification.FilterRule{},
			},
		}
		if filter.Prefix != "" {
			webhookNotification.Filter.Key.FilterRules = append(
				webhookNotification.Filter.Key.FilterRules,
				notification.FilterRule{
					Name:  "prefix",
					Value: filter.Prefix,
				},
			)
		}
		if filter.Suffix != "" {
			webhookNotification.Filter.Key.FilterRules = append(
				webhookNotification.Filter.Key.FilterRules,
				notification.FilterRule{
					Name:  "suffix",
					Value: filter.Suffix,
				},
			)
		}
	}

	bucketNotification.CloudWatchConfigs = []notification.CloudWatchConfig{webhookNotification}

	err = minioClient.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, bucketNotification)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNotification)
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// For simplicity, we'll delete and recreate
	err := c.Delete(ctx, mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	_, err = c.Create(ctx, mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1beta1.NotificationConfiguration)
	if !ok {
		return errors.New(errNotNotificationConfiguration)
	}

	webhookConfig := cr.Spec.ForProvider.WebhookConfiguration
	if webhookConfig == nil {
		return nil
	}

	// Remove webhook configuration from MinIO admin
	webhookID := webhookConfig.ID
	err := c.client.DelConfigKV(ctx, fmt.Sprintf("notify_webhook:%s", webhookID))
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		return errors.Wrap(err, errDeleteNotification)
	}

	// Create a minio-go client for bucket operations
	minioClient, err := minio.New(c.cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.cfg.AccessKey, c.cfg.SecretKey, ""),
		Secure: c.cfg.UseSSL,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create MinIO client")
	}

	// Remove bucket notification
	bucketNotification := notification.Configuration{}
	err = minioClient.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, bucketNotification)
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		return errors.Wrap(err, errDeleteNotification)
	}

	return nil
}

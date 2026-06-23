package notificationconfiguration

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7/pkg/notification"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (nc *notificationClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("creating resource")

	cr, ok := mg.(*miniov1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalCreation{}, errNotNotificationConfiguration
	}

	cr.SetConditions(xpv1.Creating())

	webhookConfig := cr.Spec.ForProvider.WebhookConfiguration
	if webhookConfig == nil {
		err := fmt.Errorf("webhook configuration is required")
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalCreation{}, err
	}

	// Get current bucket notification configuration
	config, err := nc.mc.GetBucketNotification(ctx, cr.Spec.ForProvider.BucketName)
	if err != nil {
		// If no configuration exists yet, start fresh
		config = notification.Configuration{}
	}

	// Check if webhook configuration already exists (idempotency)
	expectedARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookConfig.ID)
	for _, lambda := range config.LambdaConfigs {
		if lambda.Arn.String() == expectedARN && lambda.Lambda == webhookConfig.Endpoint {
			// Configuration already exists, nothing to do
			cr.SetConditions(xpv1.Available())
			return managed.ExternalCreation{}, nil
		}
	}

	// Create webhook configuration using LambdaConfig
	// In MinIO, webhooks are configured using LambdaConfig with the webhook URL as the Lambda field.
	lambdaConfig := notification.LambdaConfig{
		Lambda: webhookConfig.Endpoint,
	}

	// Create the base Config for the notification.
	// ARN format: arn:minio:sqs::<webhook-id>:webhook
	// This format identifies the webhook uniquely in the bucket's notification configuration.
	lambdaConfig.Config = notification.NewConfig(
		notification.NewArn("minio", "sqs", "", webhookConfig.ID, "webhook"),
	)

	// Add events
	for _, event := range cr.Spec.ForProvider.Events {
		lambdaConfig.Events = append(lambdaConfig.Events, notification.EventType(event))
	}

	// Add filter if specified.
	// Note: Only S3 Key filters are supported (filter.Key.FilterRules).
	// Other filter types (metadata, tags, etc.) are not currently implemented.
	if filter := cr.Spec.ForProvider.Filter; filter != nil && filter.Key != nil {
		lambdaConfig.Filter = &notification.Filter{
			S3Key: notification.S3Key{
				FilterRules: []notification.FilterRule{},
			},
		}
		for _, rule := range filter.Key.FilterRules {
			lambdaConfig.Filter.S3Key.FilterRules = append(
				lambdaConfig.Filter.S3Key.FilterRules,
				notification.FilterRule{
					Name:  rule.Name,
					Value: rule.Value,
				},
			)
		}
	}

	config.LambdaConfigs = append(config.LambdaConfigs, lambdaConfig)

	err = nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
	if err != nil {
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalCreation{}, err
	}

	cr.SetConditions(xpv1.Available())
	nc.emitCreationEvent(cr)

	return managed.ExternalCreation{}, nil
}

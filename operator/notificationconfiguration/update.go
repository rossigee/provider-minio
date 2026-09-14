package notificationconfiguration

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7/pkg/notification"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (nc *notificationClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("updating resource")

	cr, ok := mg.(*miniov1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalUpdate{}, errNotNotificationConfiguration
	}

	cr.SetConditions(xpv1.Creating())

	webhookConfig := cr.Spec.ForProvider.WebhookConfiguration
	if webhookConfig == nil {
		return managed.ExternalUpdate{}, nil
	}

	// Get current bucket notification configuration
	config, err := nc.mc.GetBucketNotification(ctx, cr.Spec.ForProvider.BucketName)
	if err != nil {
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalUpdate{}, err
	}

	// Remove old webhook configuration if it exists and add updated one
	// Webhook is stored as QueueConfig with static ARN (matching create.go/observe.go/delete.go)
	webhookARN := "arn:minio:sqs:us-east-1:_:webhook"
	filtered := []notification.QueueConfig{}
	for _, queue := range config.QueueConfigs {
		if queue.Queue != webhookARN {
			filtered = append(filtered, queue)
		}
	}

	// Add updated webhook configuration using QueueConfig
	webhookQueueConfig := notification.QueueConfig{
		Queue: webhookARN,
	}

	webhookQueueConfig.Config = notification.NewConfig(
		notification.NewArn("", "", "", "", ""),
	)

	for _, event := range cr.Spec.ForProvider.Events {
		webhookQueueConfig.Events = append(webhookQueueConfig.Events, notification.EventType(event))
	}

	if filter := cr.Spec.ForProvider.Filter; filter != nil && filter.Key != nil {
		webhookQueueConfig.Filter = &notification.Filter{
			S3Key: notification.S3Key{
				FilterRules: []notification.FilterRule{},
			},
		}
		for _, rule := range filter.Key.FilterRules {
			webhookQueueConfig.Filter.S3Key.FilterRules = append(
				webhookQueueConfig.Filter.S3Key.FilterRules,
				notification.FilterRule{
					Name:  rule.Name,
					Value: rule.Value,
				},
			)
		}
	}

	config.QueueConfigs = append(filtered, webhookQueueConfig)

	err = nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
	if err != nil {
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalUpdate{}, err
	}

	cr.SetConditions(xpv1.Available())
	nc.emitUpdateEvent(cr)

	return managed.ExternalUpdate{}, nil
}

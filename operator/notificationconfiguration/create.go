package notificationconfiguration

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
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
	queueConfig := cr.Spec.ForProvider.QueueConfiguration

	if webhookConfig == nil && queueConfig == nil {
		err := fmt.Errorf("either webhook or queue configuration is required")
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalCreation{}, err
	}

	// Get current bucket notification configuration
	config, err := nc.mc.GetBucketNotification(ctx, cr.Spec.ForProvider.BucketName)
	if err != nil {
		// If no configuration exists yet, start fresh
		config = notification.Configuration{}
	}

	// Check if bucket has incompatible Topic configs that would block webhooks
	if len(config.TopicConfigs) > 0 {
		err := fmt.Errorf("bucket has Topic notifications which block webhook configuration - remove them first using MinIO Client (mc)")
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalCreation{}, err
	}

	// Handle webhook configuration if specified
	// Webhooks on this MinIO server use SQS-type ARN format: arn:minio:sqs:us-east-1:_:webhook
	if webhookConfig != nil {
		webhookARN := "arn:minio:sqs:us-east-1:_:webhook"
		webhookExists := false
		for _, queue := range config.QueueConfigs {
			if queue.Arn.String() == webhookARN {
				webhookExists = true
				log.V(1).Info("webhook configuration already exists")
				break
			}
		}

		if !webhookExists {
			// Create webhook configuration using QueueConfig with webhook backend ARN
			webhookQueueConfig := notification.QueueConfig{}

			webhookQueueConfig.Config = notification.NewConfig(
				notification.NewArn("minio", "sqs", "us-east-1", "_", "webhook"),
			)

			// Add events
			for _, event := range cr.Spec.ForProvider.Events {
				webhookQueueConfig.Events = append(webhookQueueConfig.Events, notification.EventType(event))
			}

			// Add filter
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

			config.QueueConfigs = append(config.QueueConfigs, webhookQueueConfig)
		}
	}

	// Handle queue configuration if specified
	if queueConfig != nil {
		queueExists := false
		for _, queue := range config.QueueConfigs {
			if queue.Arn.String() == queueConfig.QueueArn {
				queueExists = true
				log.V(1).Info("queue configuration already exists")
				break
			}
		}

		if !queueExists {
			// Create queue configuration using QueueConfig
			qConfig := notification.QueueConfig{
				Queue: queueConfig.QueueArn,
			}

			qConfig.Config = notification.NewConfig(
				notification.NewArn("minio", "sqs", "", queueConfig.ID, "queue"),
			)

			// Add events
			for _, event := range cr.Spec.ForProvider.Events {
				qConfig.Events = append(qConfig.Events, notification.EventType(event))
			}

			// Add filter
			if filter := cr.Spec.ForProvider.Filter; filter != nil && filter.Key != nil {
				qConfig.Filter = &notification.Filter{
					S3Key: notification.S3Key{
						FilterRules: []notification.FilterRule{},
					},
				}
				for _, rule := range filter.Key.FilterRules {
					qConfig.Filter.S3Key.FilterRules = append(
						qConfig.Filter.S3Key.FilterRules,
						notification.FilterRule{
							Name:  rule.Name,
							Value: rule.Value,
						},
					)
				}
			}

			config.QueueConfigs = append(config.QueueConfigs, qConfig)
		}
	}

	// Set notification configuration (both webhook and queue if specified)
	if len(config.LambdaConfigs) > 0 || len(config.QueueConfigs) > 0 {
		err = nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
		if err != nil {
			cr.SetConditions(xpv1.ReconcileError(err))
			return managed.ExternalCreation{}, err
		}
	}

	cr.SetConditions(xpv1.Available())
	nc.emitCreationEvent(cr)

	return managed.ExternalCreation{}, nil
}

func (nc *notificationClient) emitAdoptionEvent(cr *miniov1beta1.NotificationConfiguration) {
	nc.recorder.Event(cr, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Adopted",
		Message: fmt.Sprintf("Adopted existing notification configuration for bucket %s", cr.Spec.ForProvider.BucketName),
	})
}

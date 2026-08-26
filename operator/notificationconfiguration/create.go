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
	topicConfig := cr.Spec.ForProvider.TopicConfiguration

	if webhookConfig == nil && queueConfig == nil && topicConfig == nil {
		err := fmt.Errorf("at least one notification configuration (webhook, queue, or topic) is required")
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalCreation{}, err
	}

	// Get current bucket notification configuration
	config, err := nc.mc.GetBucketNotification(ctx, cr.Spec.ForProvider.BucketName)
	if err != nil {
		// If no configuration exists yet, start fresh
		config = notification.Configuration{}
	}

	// Check if webhook configuration already exists (idempotency and adoption)
	if webhookConfig != nil {
		expectedARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookConfig.ID)
		for _, lambda := range config.LambdaConfigs {
			if lambda.Arn.String() == expectedARN {
				// Webhook exists - either exact match or adoption
				if lambda.Lambda == webhookConfig.Endpoint {
					// Exact match
					log.V(1).Info("webhook configuration already exists")
					cr.SetConditions(xpv1.Available())
					return managed.ExternalCreation{}, nil
				} else {
					// Different endpoint - adopt it anyway
					log.V(1).Info("adopting existing webhook configuration with different endpoint")
					nc.emitAdoptionEvent(cr)
					cr.SetConditions(xpv1.Available())
					return managed.ExternalCreation{}, nil
				}
			}
		}

		// Create webhook configuration using LambdaConfig
		lambdaConfig := notification.LambdaConfig{
			Lambda: webhookConfig.Endpoint,
		}

		lambdaConfig.Config = notification.NewConfig(
			notification.NewArn("minio", "sqs", "", webhookConfig.ID, "webhook"),
		)

		// Add events
		for _, event := range cr.Spec.ForProvider.Events {
			lambdaConfig.Events = append(lambdaConfig.Events, notification.EventType(event))
		}

		// Add filter
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
	}

	// Handle Queue configuration
	if queueConfig != nil {
		queueCfg := notification.QueueConfig{}
		arn, err := notification.NewArnFromString(queueConfig.QueueArn)
		if err != nil {
			cr.SetConditions(xpv1.ReconcileError(err))
			return managed.ExternalCreation{}, err
		}
		queueCfg.Config = notification.NewConfig(arn)

		// Add events
		for _, event := range cr.Spec.ForProvider.Events {
			queueCfg.Events = append(queueCfg.Events, notification.EventType(event))
		}

		// Add filter
		if filter := cr.Spec.ForProvider.Filter; filter != nil && filter.Key != nil {
			queueCfg.Filter = &notification.Filter{
				S3Key: notification.S3Key{
					FilterRules: []notification.FilterRule{},
				},
			}
			for _, rule := range filter.Key.FilterRules {
				queueCfg.Filter.S3Key.FilterRules = append(
					queueCfg.Filter.S3Key.FilterRules,
					notification.FilterRule{
						Name:  rule.Name,
						Value: rule.Value,
					},
				)
			}
		}

		config.QueueConfigs = append(config.QueueConfigs, queueCfg)
	}

	// Handle Topic configuration
	if topicConfig != nil {
		topicCfg := notification.TopicConfig{}
		arn, err := notification.NewArnFromString(topicConfig.TopicArn)
		if err != nil {
			cr.SetConditions(xpv1.ReconcileError(err))
			return managed.ExternalCreation{}, err
		}
		topicCfg.Config = notification.NewConfig(arn)

		// Add events
		for _, event := range cr.Spec.ForProvider.Events {
			topicCfg.Events = append(topicCfg.Events, notification.EventType(event))
		}

		// Add filter
		if filter := cr.Spec.ForProvider.Filter; filter != nil && filter.Key != nil {
			topicCfg.Filter = &notification.Filter{
				S3Key: notification.S3Key{
					FilterRules: []notification.FilterRule{},
				},
			}
			for _, rule := range filter.Key.FilterRules {
				topicCfg.Filter.S3Key.FilterRules = append(
					topicCfg.Filter.S3Key.FilterRules,
					notification.FilterRule{
						Name:  rule.Name,
						Value: rule.Value,
					},
				)
			}
		}

		config.TopicConfigs = append(config.TopicConfigs, topicCfg)
	}

	// Only set notification if we have at least one notification configured
	hasNotifications := len(config.LambdaConfigs) > 0 || len(config.QueueConfigs) > 0 || len(config.TopicConfigs) > 0
	if hasNotifications {
		err = nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
		if err != nil {
			// If full config failed but we have webhooks, try webhook-only as fallback
			if webhookConfig != nil && len(config.LambdaConfigs) > 0 && (len(config.QueueConfigs) > 0 || len(config.TopicConfigs) > 0) {
				log.V(1).Info("full notification config failed, retrying with webhooks only", "error", err)
				config.QueueConfigs = nil
				config.TopicConfigs = nil
				retryErr := nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
				if retryErr == nil {
					// Webhook-only succeeded, log success and continue
					log.V(1).Info("webhook-only notification config succeeded after full config failed")
					cr.SetConditions(xpv1.Available())
					nc.emitCreationEvent(cr)
					return managed.ExternalCreation{}, nil
				}
				// Webhook-only also failed, use the original error
			}
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

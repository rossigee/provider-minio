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

	// Remove old webhook configuration if it exists
	oldARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookConfig.ID)
	filtered := []notification.LambdaConfig{}
	for _, lambda := range config.LambdaConfigs {
		if lambda.Arn.String() != oldARN {
			filtered = append(filtered, lambda)
		}
	}
	config.LambdaConfigs = filtered

	// Add updated webhook configuration
	lambdaConfig := notification.LambdaConfig{
		Lambda: webhookConfig.Endpoint,
	}
	lambdaConfig.Config = notification.NewConfig(
		notification.NewArn("minio", "sqs", "", webhookConfig.ID, "webhook"),
	)

	for _, event := range cr.Spec.ForProvider.Events {
		lambdaConfig.Events = append(lambdaConfig.Events, notification.EventType(event))
	}

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
		return managed.ExternalUpdate{}, err
	}

	cr.SetConditions(xpv1.Available())
	nc.emitUpdateEvent(cr)

	return managed.ExternalUpdate{}, nil
}

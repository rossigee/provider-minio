package notificationconfiguration

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7/pkg/notification"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (nc *notificationClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("observing resource")

	cr, ok := mg.(*miniov1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalObservation{}, errNotNotificationConfiguration
	}

	bucketName := cr.Spec.ForProvider.BucketName

	config, err := nc.mc.GetBucketNotification(ctx, bucketName)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			cr.SetConditions(xpv1.Creating())
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	// Check if our webhook configuration exists and is up-to-date
	lambdaConfig, found := nc.findWebhookConfiguration(cr, &config)
	if !found {
		cr.SetConditions(xpv1.Creating())
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	upToDate := nc.isConfigurationUpToDate(cr, lambdaConfig)

	cr.Status.AtProvider.BucketName = bucketName
	if cr.Spec.ForProvider.WebhookConfiguration != nil {
		cr.Status.AtProvider.ConfigurationID = cr.Spec.ForProvider.WebhookConfiguration.ID
	}

	if upToDate {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Creating())
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (nc *notificationClient) findWebhookConfiguration(cr *miniov1beta1.NotificationConfiguration, config *notification.Configuration) (*notification.LambdaConfig, bool) {
	if cr.Spec.ForProvider.WebhookConfiguration == nil {
		return nil, false
	}

	webhookID := cr.Spec.ForProvider.WebhookConfiguration.ID
	endpoint := cr.Spec.ForProvider.WebhookConfiguration.Endpoint
	expectedARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookID)

	for i, lambda := range config.LambdaConfigs {
		if lambda.Arn.String() == expectedARN && lambda.Lambda == endpoint {
			return &config.LambdaConfigs[i], true
		}
	}

	return nil, false
}

func (nc *notificationClient) isConfigurationUpToDate(cr *miniov1beta1.NotificationConfiguration, lambdaConfig *notification.LambdaConfig) bool {
	if lambdaConfig == nil || cr.Spec.ForProvider.WebhookConfiguration == nil {
		return false
	}

	// Check endpoint matches
	if lambdaConfig.Lambda != cr.Spec.ForProvider.WebhookConfiguration.Endpoint {
		return false
	}

	// Check events match
	specEvents := make(map[string]bool)
	for _, event := range cr.Spec.ForProvider.Events {
		specEvents[event] = true
	}

	if len(lambdaConfig.Events) != len(specEvents) {
		return false
	}

	for _, event := range lambdaConfig.Events {
		if !specEvents[string(event)] {
			return false
		}
	}

	// Check filter rules match if specified
	if cr.Spec.ForProvider.Filter != nil && cr.Spec.ForProvider.Filter.Key != nil {
		if lambdaConfig.Filter == nil || lambdaConfig.Filter.S3Key.FilterRules == nil {
			return false
		}

		if len(lambdaConfig.Filter.S3Key.FilterRules) != len(cr.Spec.ForProvider.Filter.Key.FilterRules) {
			return false
		}

		for i, rule := range cr.Spec.ForProvider.Filter.Key.FilterRules {
			if i >= len(lambdaConfig.Filter.S3Key.FilterRules) {
				return false
			}
			if lambdaConfig.Filter.S3Key.FilterRules[i].Name != rule.Name ||
				lambdaConfig.Filter.S3Key.FilterRules[i].Value != rule.Value {
				return false
			}
		}
	} else if lambdaConfig.Filter != nil {
		// Filter expected to be nil but found one
		return false
	}

	return true
}

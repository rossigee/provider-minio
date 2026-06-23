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

func (nc *notificationClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("deleting resource")

	cr, ok := mg.(*miniov1beta1.NotificationConfiguration)
	if !ok {
		return managed.ExternalDelete{}, errNotNotificationConfiguration
	}

	cr.SetConditions(xpv1.Deleting())

	webhookConfig := cr.Spec.ForProvider.WebhookConfiguration
	if webhookConfig == nil {
		return managed.ExternalDelete{}, nil
	}

	// Get current bucket notification configuration
	config, err := nc.mc.GetBucketNotification(ctx, cr.Spec.ForProvider.BucketName)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return managed.ExternalDelete{}, nil
		}
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalDelete{}, err
	}

	// Remove our webhook configuration from the bucket
	webhookID := webhookConfig.ID
	expectedARN := fmt.Sprintf("arn:minio:sqs::%s:webhook", webhookID)

	filtered := []notification.LambdaConfig{}
	for _, lambda := range config.LambdaConfigs {
		if lambda.Arn.String() != expectedARN || lambda.Lambda != webhookConfig.Endpoint {
			filtered = append(filtered, lambda)
		}
	}
	config.LambdaConfigs = filtered

	// Update bucket notification
	err = nc.mc.SetBucketNotification(ctx, cr.Spec.ForProvider.BucketName, config)
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalDelete{}, err
	}

	nc.emitDeletionEvent(cr)

	return managed.ExternalDelete{}, nil
}

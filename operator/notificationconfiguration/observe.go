package notificationconfiguration

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
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

	// Check if bucket has incompatible Topic configs that would block webhooks
	if len(config.TopicConfigs) > 0 {
		err := fmt.Errorf("bucket has Topic notifications which block webhook configuration - remove them first using MinIO Client (mc)")
		cr.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	// Check if our webhook configuration exists and is up-to-date
	webhookExists := false
	if cr.Spec.ForProvider.WebhookConfiguration != nil {
		webhookARN := "arn:minio:sqs:us-east-1:_:webhook"
		for _, queue := range config.QueueConfigs {
			if queue.Queue == webhookARN {
				webhookExists = true
				log.V(1).Info("webhook configuration already exists")
				break
			}
		}
	}

	if !webhookExists && cr.Spec.ForProvider.WebhookConfiguration != nil {
		cr.SetConditions(xpv1.Creating())
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.BucketName = bucketName
	if cr.Spec.ForProvider.WebhookConfiguration != nil {
		cr.Status.AtProvider.ConfigurationID = cr.Spec.ForProvider.WebhookConfiguration.ID
	}

	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

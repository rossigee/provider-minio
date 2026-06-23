package notificationconfiguration

import (
	"context"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ admission.Validator[*miniov1beta1.NotificationConfiguration] = &Validator{}
)

// Validator validates admission requests.
type Validator struct {
	log  logr.Logger
	kube client.Client
}

// ValidateCreate implements admission.Validator.
func (v *Validator) ValidateCreate(ctx context.Context, nc *miniov1beta1.NotificationConfiguration) (admission.Warnings, error) {
	v.log.V(1).Info("Validate create")

	providerConfigRef := nc.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	if nc.Spec.ForProvider.BucketName == "" {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "bucketName"), "", "Bucket name is required")
	}

	if len(nc.Spec.ForProvider.Events) == 0 {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "events"), nil, "At least one event is required")
	}

	if err := ValidateEvents(nc.Spec.ForProvider.Events); err != nil {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "events"), nc.Spec.ForProvider.Events, err.Error())
	}

	if nc.Spec.ForProvider.WebhookConfiguration == nil {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "webhookConfiguration"), nil, "Webhook configuration is required")
	}

	if nc.Spec.ForProvider.WebhookConfiguration.Endpoint == "" {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "webhookConfiguration", "endpoint"), "", "Endpoint is required")
	}

	if nc.Spec.ForProvider.WebhookConfiguration.ID == "" {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "webhookConfiguration", "id"), "", "ID is required")
	}

	return nil, nil
}

// ValidateUpdate implements admission.Validator.
func (v *Validator) ValidateUpdate(ctx context.Context, oldNC, newNC *miniov1beta1.NotificationConfiguration) (admission.Warnings, error) {
	v.log.V(1).Info("Validate update")

	if newNC.Spec.ForProvider.BucketName != oldNC.Spec.ForProvider.BucketName {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "bucketName"), newNC.Spec.ForProvider.BucketName, "Changing the bucket name is not allowed")
	}

	providerConfigRef := newNC.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	if newNC.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	if len(newNC.Spec.ForProvider.Events) == 0 {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "events"), nil, "At least one event is required")
	}

	return nil, nil
}

// ValidateDelete implements admission.Validator.
func (v *Validator) ValidateDelete(_ context.Context, _ *miniov1beta1.NotificationConfiguration) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

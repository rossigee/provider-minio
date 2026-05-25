package bucket

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.Validator[*miniov1beta1.Bucket] = &Validator{}

// Validator validates admission requests.
type Validator struct {
	log logr.Logger
}

// ValidateCreate implements admission.Validator.
func (v *Validator) ValidateCreate(_ context.Context, bucket *miniov1beta1.Bucket) (admission.Warnings, error) {
	v.log.V(1).Info("Validate create", "name", bucket.Name)
	providerConfigRef := bucket.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, fmt.Errorf(".spec.providerConfigRef.name is required")
	}
	return nil, nil
}

// ValidateUpdate implements admission.Validator.
func (v *Validator) ValidateUpdate(_ context.Context, oldBucket, newBucket *miniov1beta1.Bucket) (admission.Warnings, error) {
	v.log.V(1).Info("Validate update")

	if oldBucket.Status.AtProvider.BucketName != "" {
		if newBucket.GetBucketName() != oldBucket.Status.AtProvider.BucketName {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "bucketName"), newBucket.Spec.ForProvider.BucketName, "Changing the bucket name is not allowed after creation")
		}
		if newBucket.Spec.ForProvider.Region != oldBucket.Spec.ForProvider.Region {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "region"), newBucket.Spec.ForProvider.Region, "Changing the region is not allowed after creation")
		}
	}
	providerConfigRef := newBucket.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}
	return nil, nil
}

// ValidateDelete implements admission.Validator.
func (v *Validator) ValidateDelete(_ context.Context, _ *miniov1beta1.Bucket) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

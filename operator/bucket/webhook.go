package bucket

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.CustomValidator = &Validator{}

// Validator validates admission requests.
type Validator struct {
	log logr.Logger
}

// ValidateCreate implements admission.CustomValidator.
func (v *Validator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Handle both v1 and v1beta1 API versions
	if bucketv1, ok := obj.(*miniov1.Bucket); ok {
		v.log.V(1).Info("Validate create v1", "name", bucketv1.Name)
		providerConfigRef := bucketv1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, fmt.Errorf(".spec.providerConfigRef.name is required")
		}
		return nil, nil
	}

	if bucketv1beta1, ok := obj.(*miniov1beta1.Bucket); ok {
		v.log.V(1).Info("Validate create v1beta1", "name", bucketv1beta1.Name)
		providerConfigRef := bucketv1beta1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, fmt.Errorf(".spec.providerConfigRef.name is required")
		}
		return nil, nil
	}

	return nil, errNotBucket
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Handle both v1 and v1beta1 API versions
	if newBucketv1, ok := newObj.(*miniov1.Bucket); ok {
		oldBucketv1 := oldObj.(*miniov1.Bucket)
		v.log.V(1).Info("Validate update v1")

		if oldBucketv1.Status.AtProvider.BucketName != "" {
			if newBucketv1.GetBucketName() != oldBucketv1.Status.AtProvider.BucketName {
				return nil, field.Invalid(field.NewPath("spec", "forProvider", "bucketName"), newBucketv1.Spec.ForProvider.BucketName, "Changing the bucket name is not allowed after creation")
			}
			if newBucketv1.Spec.ForProvider.Region != oldBucketv1.Spec.ForProvider.Region {
				return nil, field.Invalid(field.NewPath("spec", "forProvider", "region"), newBucketv1.Spec.ForProvider.Region, "Changing the region is not allowed after creation")
			}
		}
		providerConfigRef := newBucketv1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}
		return nil, nil
	}

	if newBucketv1beta1, ok := newObj.(*miniov1beta1.Bucket); ok {
		oldBucketv1beta1 := oldObj.(*miniov1beta1.Bucket)
		v.log.V(1).Info("Validate update v1beta1")

		if oldBucketv1beta1.Status.AtProvider.BucketName != "" {
			if newBucketv1beta1.GetBucketName() != oldBucketv1beta1.Status.AtProvider.BucketName {
				return nil, field.Invalid(field.NewPath("spec", "forProvider", "bucketName"), newBucketv1beta1.Spec.ForProvider.BucketName, "Changing the bucket name is not allowed after creation")
			}
			if newBucketv1beta1.Spec.ForProvider.Region != oldBucketv1beta1.Spec.ForProvider.Region {
				return nil, field.Invalid(field.NewPath("spec", "forProvider", "region"), newBucketv1beta1.Spec.ForProvider.Region, "Changing the region is not allowed after creation")
			}
		}
		providerConfigRef := newBucketv1beta1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}
		return nil, nil
	}

	return nil, errNotBucket
}

// ValidateDelete implements admission.CustomValidator.
func (v *Validator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

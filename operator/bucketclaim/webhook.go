package bucketclaim

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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
	bucketClaim, ok := obj.(*miniov1beta1.BucketClaim)
	if !ok {
		return nil, errNotBucketClaim
	}

	v.log.V(1).Info("Validate create", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)
	credentialsSecretRef := bucketClaim.Spec.CredentialsSecretRef
	if credentialsSecretRef == nil || credentialsSecretRef.Name == "" {
		return nil, fmt.Errorf(".spec.credentialsSecretRef.name is required")
	}
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newBucketClaim, ok := newObj.(*miniov1beta1.BucketClaim)
	if !ok {
		return nil, errNotBucketClaim
	}

	oldBucketClaim := oldObj.(*miniov1beta1.BucketClaim)
	v.log.V(1).Info("Validate update", "name", newBucketClaim.Name, "namespace", newBucketClaim.Namespace)

	if oldBucketClaim.Status.AtProvider.BucketName != "" {
		if newBucketClaim.GetBucketName() != oldBucketClaim.Status.AtProvider.BucketName {
			return nil, field.Invalid(field.NewPath("spec", "bucketName"), newBucketClaim.Spec.BucketName, "Changing the bucket name is not allowed after creation")
		}
		if newBucketClaim.Spec.Region != oldBucketClaim.Spec.Region {
			return nil, field.Invalid(field.NewPath("spec", "region"), newBucketClaim.Spec.Region, "Changing the region is not allowed after creation")
		}
	}
	credentialsSecretRef := newBucketClaim.Spec.CredentialsSecretRef
	if credentialsSecretRef == nil || credentialsSecretRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "credentialsSecretRef", "name"), "null", "Credentials secret reference is required")
	}
	return nil, nil
}

// ValidateDelete implements admission.CustomValidator.
func (v *Validator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

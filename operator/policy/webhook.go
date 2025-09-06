package policy

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
	if policyv1, ok := obj.(*miniov1.Policy); ok {
		v.log.V(1).Info("Validate create v1")
		return nil, v.validatePolicy(policyv1)
	}
	
	if policyv1beta1, ok := obj.(*miniov1beta1.Policy); ok {
		v.log.V(1).Info("Validate create v1beta1")
		return nil, v.validatePolicyV1Beta1(policyv1beta1)
	}
	
	return nil, errNotPolicy
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	// Handle both v1 and v1beta1 API versions
	if newPolicyv1, ok := newObj.(*miniov1.Policy); ok {
		v.log.V(1).Info("Validate update v1")
		return nil, v.validatePolicy(newPolicyv1)
	}
	
	if newPolicyv1beta1, ok := newObj.(*miniov1beta1.Policy); ok {
		v.log.V(1).Info("Validate update v1beta1")
		return nil, v.validatePolicyV1Beta1(newPolicyv1beta1)
	}
	
	return nil, errNotPolicy
}

// ValidateDelete implements admission.CustomValidator.
func (v *Validator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

func (v *Validator) validatePolicy(policy *miniov1.Policy) error {
	if policy.Spec.ForProvider.AllowBucket != "" && policy.Spec.ForProvider.RawPolicy != "" {
		return fmt.Errorf(".spec.forProvider.allowBucket and .spec.forProvider.rawPolicy are mutual exclusive, please only specify one")
	}

	providerConfigRef := policy.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}
	return nil
}

func (v *Validator) validatePolicyV1Beta1(policy *miniov1beta1.Policy) error {
	if policy.Spec.ForProvider.AllowBucket != "" && policy.Spec.ForProvider.RawPolicy != "" {
		return fmt.Errorf(".spec.forProvider.allowBucket and .spec.forProvider.rawPolicy are mutual exclusive, please only specify one")
	}

	providerConfigRef := policy.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}
	return nil
}

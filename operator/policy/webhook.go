package policy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.Validator[*miniov1beta1.Policy] = &Validator{}

// Validator validates admission requests.
type Validator struct {
	log logr.Logger
}

// ValidateCreate implements admission.Validator.
func (v *Validator) ValidateCreate(_ context.Context, policy *miniov1beta1.Policy) (admission.Warnings, error) {
	v.log.V(1).Info("Validate create")
	return nil, v.validatePolicy(policy)
}

// ValidateUpdate implements admission.Validator.
func (v *Validator) ValidateUpdate(_ context.Context, _, newPolicy *miniov1beta1.Policy) (admission.Warnings, error) {
	v.log.V(1).Info("Validate update")
	return nil, v.validatePolicy(newPolicy)
}

// ValidateDelete implements admission.Validator.
func (v *Validator) ValidateDelete(_ context.Context, _ *miniov1beta1.Policy) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

func (v *Validator) validatePolicy(policy *miniov1beta1.Policy) error {
	if policy.Spec.ForProvider.AllowBucket != "" && policy.Spec.ForProvider.RawPolicy != "" {
		return fmt.Errorf(".spec.forProvider.allowBucket and .spec.forProvider.rawPolicy are mutual exclusive, please only specify one")
	}

	providerConfigRef := policy.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}
	return nil
}

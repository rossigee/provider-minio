package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ admission.CustomValidator = &Validator{}
)

// Validator validates admission requests.
type Validator struct {
	log  logr.Logger
	kube client.Client
}

// ValidateCreate implements admission.CustomValidator.
func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("Validate create")

	serviceAccount, ok := obj.(*miniov1beta1.ServiceAccount)
	if !ok {
		return nil, errNotServiceAccount
	}

	providerConfigRef := serviceAccount.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	// Validate policy if specified
	if serviceAccount.Spec.ForProvider.Policy != "" {
		err := v.validatePolicy(ctx, serviceAccount, serviceAccount.Spec.ForProvider.Policy)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policy"), serviceAccount.Spec.ForProvider.Policy, err.Error())
		}
	}

	// Validate access key format if specified
	if serviceAccount.Spec.ForProvider.AccessKey != "" {
		if len(serviceAccount.Spec.ForProvider.AccessKey) < 3 || len(serviceAccount.Spec.ForProvider.AccessKey) > 128 {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "accessKey"), serviceAccount.Spec.ForProvider.AccessKey, "Access key must be between 3 and 128 characters")
		}
	}

	// Validate secret key format if specified
	if serviceAccount.Spec.ForProvider.SecretKey != "" {
		if len(serviceAccount.Spec.ForProvider.SecretKey) < 8 {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "secretKey"), "***", "Secret key must be at least 8 characters")
		}
	}

	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("Validate update")

	oldServiceAccount, ok := oldObj.(*miniov1beta1.ServiceAccount)
	if !ok {
		return nil, errNotServiceAccount
	}
	newServiceAccount, ok := newObj.(*miniov1beta1.ServiceAccount)
	if !ok {
		return nil, errNotServiceAccount
	}

	// Check if immutable fields have changed
	if newServiceAccount.GetAccessKey() != oldServiceAccount.GetAccessKey() {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "accessKey"), newServiceAccount.GetAccessKey(), "Changing the access key is not allowed")
	}

	if newServiceAccount.Spec.ForProvider.TargetUser != oldServiceAccount.Spec.ForProvider.TargetUser {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "targetUser"), newServiceAccount.Spec.ForProvider.TargetUser, "Changing the target user is not allowed")
	}

	providerConfigRef := newServiceAccount.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	// Skip validation if the service account is being deleted
	if newServiceAccount.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	// Validate policy if specified
	if newServiceAccount.Spec.ForProvider.Policy != "" {
		err := v.validatePolicy(ctx, newServiceAccount, newServiceAccount.Spec.ForProvider.Policy)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policy"), newServiceAccount.Spec.ForProvider.Policy, err.Error())
		}
	}

	return nil, nil
}

// ValidateDelete implements admission.CustomValidator.
func (v *Validator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

func (v *Validator) validatePolicy(ctx context.Context, serviceAccount *miniov1beta1.ServiceAccount, policy string) error {
	// Empty policy is valid (means inherit from parent user)
	if policy == "" {
		return nil
	}

	// Validate that the policy is valid JSON
	var policyDoc interface{}
	if err := json.Unmarshal([]byte(policy), &policyDoc); err != nil {
		return fmt.Errorf("policy must be valid JSON: %w", err)
	}

	// You could add more sophisticated policy validation here
	// For example, validating the policy structure according to MinIO IAM policy format

	return nil
}

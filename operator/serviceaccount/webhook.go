package serviceaccount

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator validates ServiceAccount resources.
type Validator struct {
	log  logr.Logger
	kube client.Client
}

var _ webhook.CustomValidator = &Validator{}

func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	serviceAccount, ok := obj.(*miniov1.ServiceAccount)
	if !ok {
		return nil, fmt.Errorf("expected ServiceAccount, got %T", obj)
	}

	log := v.log.WithValues("serviceAccount", serviceAccount.Name)
	log.V(1).Info("validating create")

	allErrs := v.validateServiceAccount(ctx, serviceAccount)
	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		miniov1.ServiceAccountGroupVersionKind.GroupKind(),
		serviceAccount.Name,
		allErrs,
	)
}

func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newServiceAccount, ok := newObj.(*miniov1.ServiceAccount)
	if !ok {
		return nil, fmt.Errorf("expected ServiceAccount, got %T", newObj)
	}

	oldServiceAccount, ok := oldObj.(*miniov1.ServiceAccount)
	if !ok {
		return nil, fmt.Errorf("expected ServiceAccount, got %T", oldObj)
	}

	log := v.log.WithValues("serviceAccount", newServiceAccount.Name)
	log.V(1).Info("validating update")

	allErrs := v.validateServiceAccount(ctx, newServiceAccount)

	// Check for immutable field changes
	if oldServiceAccount.Spec.ForProvider.ParentUser != newServiceAccount.Spec.ForProvider.ParentUser {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "forProvider", "parentUser"),
			newServiceAccount.Spec.ForProvider.ParentUser,
			"parentUser cannot be changed after creation",
		))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		miniov1.ServiceAccountGroupVersionKind.GroupKind(),
		newServiceAccount.Name,
		allErrs,
	)
}

func (v *Validator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for delete
	return nil, nil
}

func (v *Validator) validateServiceAccount(ctx context.Context, serviceAccount *miniov1.ServiceAccount) field.ErrorList {
	var allErrs field.ErrorList

	// Validate parent user is specified
	if serviceAccount.Spec.ForProvider.ParentUser == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "forProvider", "parentUser"),
			"parentUser is required",
		))
	}

	// Validate service account name if specified
	if serviceAccount.Spec.ForProvider.ServiceAccountName != "" {
		if len(serviceAccount.Spec.ForProvider.ServiceAccountName) < 3 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "forProvider", "serviceAccountName"),
				serviceAccount.Spec.ForProvider.ServiceAccountName,
				"serviceAccountName must be at least 3 characters long",
			))
		}
	}

	// Validate expiry time if specified
	if serviceAccount.Spec.ForProvider.Expiry != nil {
		if serviceAccount.Spec.ForProvider.Expiry.Time.IsZero() {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "forProvider", "expiry"),
				serviceAccount.Spec.ForProvider.Expiry,
				"expiry time cannot be zero",
			))
		}
	}

	return allErrs
}

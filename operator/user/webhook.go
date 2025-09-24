package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_                   admission.CustomValidator = &Validator{}
	getProviderConfigFn                           = getProviderConfig
	getMinioAdminFn                               = getMinioAdmin
)

type cannedPolicyLister interface {
	ListCannedPolicies(context.Context) (map[string]json.RawMessage, error)
}

// Validator validates admission requests.
type Validator struct {
	log  logr.Logger
	kube client.Client
}

// ValidateCreate implements admission.CustomValidator.
func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Handle both v1 and v1beta1 API versions
	if userv1, ok := obj.(*miniov1.User); ok {
		v.log.V(1).Info("Validate create v1")

		providerConfigRef := userv1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}

		err := v.doesPolicyExist(ctx, userv1)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), userv1.Spec.ForProvider.Policies, err.Error())
		}

		return nil, nil
	}

	if userv1beta1, ok := obj.(*miniov1beta1.User); ok {
		v.log.V(1).Info("Validate create v1beta1")

		providerConfigRef := userv1beta1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}

		err := v.doesPolicyExistV1Beta1(ctx, userv1beta1)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), userv1beta1.Spec.ForProvider.Policies, err.Error())
		}

		return nil, nil
	}

	return nil, errNotUser
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Handle both v1 and v1beta1 API versions
	if oldUserv1, ok := oldObj.(*miniov1.User); ok {
		newUserv1 := newObj.(*miniov1.User)
		v.log.V(1).Info("Validate update v1")

		if newUserv1.GetUserName() != oldUserv1.GetUserName() {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "userName"), newUserv1.GetUserName(), "Changing the username is not allowed")
		}

		providerConfigRef := newUserv1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}

		if newUserv1.GetDeletionTimestamp() != nil {
			return nil, nil
		}

		err := v.doesPolicyExist(ctx, newUserv1)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), newUserv1.Spec.ForProvider.Policies, err.Error())
		}

		return nil, nil
	}

	if oldUserv1beta1, ok := oldObj.(*miniov1beta1.User); ok {
		newUserv1beta1 := newObj.(*miniov1beta1.User)
		v.log.V(1).Info("Validate update v1beta1")

		if newUserv1beta1.GetUserName() != oldUserv1beta1.GetUserName() {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "userName"), newUserv1beta1.GetUserName(), "Changing the username is not allowed")
		}

		providerConfigRef := newUserv1beta1.Spec.ProviderConfigReference
		if providerConfigRef == nil || providerConfigRef.Name == "" {
			return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
		}

		if newUserv1beta1.GetDeletionTimestamp() != nil {
			return nil, nil
		}

		err := v.doesPolicyExistV1Beta1(ctx, newUserv1beta1)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), newUserv1beta1.Spec.ForProvider.Policies, err.Error())
		}

		return nil, nil
	}

	return nil, errNotUser
}

// ValidateDelete implements admission.CustomValidator.
func (v *Validator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

func (v *Validator) doesPolicyExist(ctx context.Context, user *miniov1.User) error {

	if len(user.Spec.ForProvider.Policies) == 0 {
		return nil
	}

	config, err := getProviderConfigFn(ctx, user, v.kube)
	if err != nil {
		return err
	}

	ma, err := getMinioAdminFn(ctx, v.kube, config)
	if err != nil {
		return err
	}

	policies, err := ma.ListCannedPolicies(ctx)
	if err != nil {
		return err
	}

	for _, policy := range user.Spec.ForProvider.Policies {
		_, ok := policies[policy]
		if !ok {
			return fmt.Errorf("policy not found: %s", policy)
		}
	}

	return nil
}

func (v *Validator) doesPolicyExistV1Beta1(ctx context.Context, user *miniov1beta1.User) error {
	if len(user.Spec.ForProvider.Policies) == 0 {
		return nil
	}
	config, err := getProviderConfigV1Beta1(ctx, user, v.kube)
	if err != nil {
		return err
	}
	ma, err := getMinioAdminFn(ctx, v.kube, config)
	if err != nil {
		return err
	}
	policies, err := ma.ListCannedPolicies(ctx)
	if err != nil {
		return err
	}
	for _, policy := range user.Spec.ForProvider.Policies {
		_, ok := policies[policy]
		if !ok {
			return fmt.Errorf("policy not found: %s", policy)
		}
	}
	return nil
}

func getProviderConfig(ctx context.Context, user *miniov1.User, kube client.Client) (*providerv1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

func getProviderConfigV1Beta1(ctx context.Context, user *miniov1beta1.User, kube client.Client) (*providerv1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

func getMinioAdmin(ctx context.Context, kube client.Client, config *providerv1.ProviderConfig) (cannedPolicyLister, error) {
	return minioutil.NewMinioAdmin(ctx, kube, config)
}

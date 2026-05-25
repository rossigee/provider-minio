package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_                   admission.Validator[*miniov1beta1.User] = &Validator{}
	getProviderConfigFn                                         = getProviderConfig
	getMinioAdminFn                                             = getMinioAdmin
)

type cannedPolicyLister interface {
	ListCannedPolicies(context.Context) (map[string]json.RawMessage, error)
}

// Validator validates admission requests.
type Validator struct {
	log  logr.Logger
	kube client.Client
}

// ValidateCreate implements admission.Validator.
func (v *Validator) ValidateCreate(ctx context.Context, user *miniov1beta1.User) (admission.Warnings, error) {
	v.log.V(1).Info("Validate create")

	providerConfigRef := user.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	if err := v.doesPolicyExist(ctx, user); err != nil {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), user.Spec.ForProvider.Policies, err.Error())
	}

	return nil, nil
}

// ValidateUpdate implements admission.Validator.
func (v *Validator) ValidateUpdate(ctx context.Context, oldUser, newUser *miniov1beta1.User) (admission.Warnings, error) {
	v.log.V(1).Info("Validate update")

	if newUser.GetUserName() != oldUser.GetUserName() {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "userName"), newUser.GetUserName(), "Changing the username is not allowed")
	}

	providerConfigRef := newUser.Spec.ProviderConfigReference
	if providerConfigRef == nil || providerConfigRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "providerConfigRef", "name"), "null", "Provider config is required")
	}

	if newUser.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	if err := v.doesPolicyExist(ctx, newUser); err != nil {
		return nil, field.Invalid(field.NewPath("spec", "forProvider", "policies"), newUser.Spec.ForProvider.Policies, err.Error())
	}

	return nil, nil
}

// ValidateDelete implements admission.Validator.
func (v *Validator) ValidateDelete(_ context.Context, _ *miniov1beta1.User) (admission.Warnings, error) {
	v.log.V(1).Info("validate delete (noop)")
	return nil, nil
}

func (v *Validator) doesPolicyExist(ctx context.Context, user *miniov1beta1.User) error {
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
		if _, ok := policies[policy]; !ok {
			return fmt.Errorf("policy not found: %s", policy)
		}
	}

	return nil
}

func getProviderConfig(ctx context.Context, user *miniov1beta1.User, kube client.Client) (*providerv1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

func getMinioAdmin(ctx context.Context, kube client.Client, config *providerv1.ProviderConfig) (cannedPolicyLister, error) {
	return minioutil.NewMinioAdmin(ctx, kube, config)
}

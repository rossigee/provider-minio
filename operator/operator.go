package operator

import (
	"github.com/rossigee/provider-minio/operator/bucket"
	"github.com/rossigee/provider-minio/operator/config"
	"github.com/rossigee/provider-minio/operator/policy"
	"github.com/rossigee/provider-minio/operator/serviceaccount"
	"github.com/rossigee/provider-minio/operator/user"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupControllers creates all controllers and adds them to the supplied manager.
func SetupControllers(mgr ctrl.Manager) error {
	for _, setup := range []func(ctrl.Manager) error{
		// bucket.SetupController,          // Disabled: only using v1beta1
		// user.SetupController,            // Disabled: only using v1beta1
		// policy.SetupController,          // Disabled: only using v1beta1
		// serviceaccount.SetupController,  // Disabled: only using v1beta1
		config.SetupController,
		// v1beta1 controllers for namespaced resources
		bucket.SetupV1Beta1Controller,
		user.SetupV1Beta1Controller,
		policy.SetupV1Beta1Controller,
		serviceaccount.SetupV1Beta1Controller,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}

// SetupWebhooks creates all webhooks and adds them to the supplied manager.
func SetupWebhooks(mgr ctrl.Manager) error {
	for _, setup := range []func(ctrl.Manager) error{
		// bucket.SetupWebhook,          // Disabled: only using v1beta1
		// user.SetupWebhook,            // Disabled: only using v1beta1
		// policy.SetupWebhook,          // Disabled: only using v1beta1
		// serviceaccount.SetupWebhook,  // Disabled: only using v1beta1
		// v1beta1 webhooks for namespaced resources
		bucket.SetupV1Beta1Webhook,
		user.SetupV1Beta1Webhook,
		policy.SetupV1Beta1Webhook,
		serviceaccount.SetupV1Beta1Webhook,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}

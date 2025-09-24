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
		config.SetupController,
		// v1beta1 controllers for namespaced resources (now the main controllers)
		bucket.SetupController,
		user.SetupController,
		policy.SetupController,
		serviceaccount.SetupController,
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
		// v1beta1 webhooks for namespaced resources (now the main webhooks)
		bucket.SetupWebhook,
		user.SetupWebhook,
		policy.SetupWebhook,
		serviceaccount.SetupWebhook,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}

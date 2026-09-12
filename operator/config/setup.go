package config

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupController adds a controller that reconciles ProviderConfigs and tracks
// their current usage.
func SetupController(mgr ctrl.Manager) error {
	name := providerconfig.ControllerName(providerv1beta1.ProviderConfigGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorder(name))

	of := resource.ProviderConfigKinds{
		Config:    providerv1beta1.ProviderConfigGroupVersionKind,
		Usage:     providerv1beta1.ProviderConfigUsageGroupVersionKind,
		UsageList: providerv1beta1.ProviderConfigUsageListGroupVersionKind,
	}

	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		providerconfig.WithRecorder(recorder))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&providerv1beta1.ProviderConfig{}).
		Watches(&providerv1beta1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(r)
}

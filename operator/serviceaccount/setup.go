package serviceaccount

import (
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupController adds a controller that reconciles managed resources.
func SetupController(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1beta1.ServiceAccountGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorder(name))

	return SetupControllerWithConnector(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
		usage:    resource.NewProviderConfigUsageTracker(mgr.GetClient(), &providerv1.ProviderConfigUsage{}),
	}, 0*time.Second)
}

func SetupControllerWithConnector(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) error {
	r := createReconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1beta1.ServiceAccount{}).
		Complete(r)
}

func createReconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) *managed.Reconciler {

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1beta1.ServiceAccountGroupVersionKind),
		managed.WithExternalConnector(c),
		managed.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		managed.WithRecorder(recorder),
		managed.WithPollInterval(1*time.Minute),
		managed.WithCreationGracePeriod(creationGracePeriod))
}

// SetupWebhook adds a webhook for managed resources.
func SetupWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &miniov1beta1.ServiceAccount{}).
		WithValidator(&Validator{
			log:  mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1beta1.ServiceAccountKind)),
			kube: mgr.GetClient(),
		}).
		Complete()
}

// SetupV1Beta1Controller adds a controller that reconciles v1beta1 managed resources.
func SetupV1Beta1Controller(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1beta1.ServiceAccountGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorder(name))

	return SetupV1Beta1ControllerWithConnector(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
		usage:    resource.NewProviderConfigUsageTracker(mgr.GetClient(), &providerv1.ProviderConfigUsage{}),
	}, 0*time.Second)
}

func SetupV1Beta1ControllerWithConnector(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) error {
	r := createV1Beta1Reconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1beta1.ServiceAccount{}).
		Complete(r)
}

func createV1Beta1Reconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) *managed.Reconciler {

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1beta1.ServiceAccountGroupVersionKind),
		managed.WithExternalConnector(c),
		managed.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		managed.WithRecorder(recorder),
		managed.WithPollInterval(1*time.Minute),
		managed.WithCreationGracePeriod(creationGracePeriod))
}

// SetupV1Beta1Webhook adds a webhook for v1beta1 managed resources.
func SetupV1Beta1Webhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &miniov1beta1.ServiceAccount{}).
		WithValidator(&Validator{
			log:  mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1beta1.ServiceAccountKind)),
			kube: mgr.GetClient(),
		}).
		Complete()
}

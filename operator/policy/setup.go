package policy

import (
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupController adds a controller that reconciles managed resources.
func SetupController(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1.PolicyGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	return SetupControllerWithConnecter(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
		usage:    resource.NewProviderConfigUsageTracker(mgr.GetClient(), &providerv1.ProviderConfigUsage{}),
	}, 0*time.Second)
}

func SetupControllerWithConnecter(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) error {
	r := createReconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1.Policy{}).
		Complete(r)
}

func createReconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) *managed.Reconciler {
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1.PolicyGroupVersionKind),
		managed.WithExternalConnecter(c),
		managed.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		managed.WithRecorder(recorder),
		managed.WithPollInterval(1*time.Minute),
		managed.WithConnectionPublishers(cps...),
		managed.WithCreationGracePeriod(creationGracePeriod))
}

// SetupWebhook adds a webhook for managed resources.
func SetupWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&miniov1.Policy{}).
		WithValidator(&Validator{
			log: mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1.PolicyKind)),
		}).
		Complete()
}

// SetupV1Beta1Controller adds a controller that reconciles v1beta1 managed resources.
func SetupV1Beta1Controller(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1beta1.PolicyGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	return SetupV1Beta1ControllerWithConnecter(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
		usage:    resource.NewProviderConfigUsageTracker(mgr.GetClient(), &providerv1.ProviderConfigUsage{}),
	}, 0*time.Second)
}

func SetupV1Beta1ControllerWithConnecter(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) error {
	r := createV1Beta1Reconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1beta1.Policy{}).
		Complete(r)
}

func createV1Beta1Reconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) *managed.Reconciler {
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1beta1.PolicyGroupVersionKind),
		managed.WithExternalConnecter(c),
		managed.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		managed.WithRecorder(recorder),
		managed.WithPollInterval(1*time.Minute),
		managed.WithConnectionPublishers(cps...),
		managed.WithCreationGracePeriod(creationGracePeriod))
}

// SetupV1Beta1Webhook adds a webhook for v1beta1 managed resources.
func SetupV1Beta1Webhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&miniov1beta1.Policy{}).
		WithValidator(&Validator{
			log: mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1beta1.PolicyKind)),
		}).
		Complete()
}

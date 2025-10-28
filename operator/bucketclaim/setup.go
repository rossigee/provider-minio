package bucketclaim

import (
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupController adds a controller that reconciles managed resources.
func SetupController(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1beta1.BucketClaimGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	return SetupControllerWithConnecter(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
	}, 0*time.Second)
}

func SetupControllerWithConnecter(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) error {
	r := createReconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1beta1.BucketClaim{}).
		Complete(r)
}

func createReconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnecter, creationGracePeriod time.Duration) *managed.Reconciler {
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1beta1.BucketClaimGroupVersionKind),
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
		For(&miniov1beta1.BucketClaim{}).
		WithValidator(&Validator{
			log: mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1beta1.BucketClaimKind)),
		}).
		Complete()
}

package bucketclaim

import (
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupController adds a controller that reconciles managed resources.
func SetupController(mgr ctrl.Manager) error {
	name := strings.ToLower(miniov1beta1.BucketClaimGroupKind)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorder(name))

	return SetupControllerWithConnector(mgr, name, recorder, &connector{
		kube:     mgr.GetClient(),
		recorder: recorder,
	}, 0*time.Second)
}

func SetupControllerWithConnector(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) error {
	r := createReconciler(mgr, name, recorder, c, creationGracePeriod)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&miniov1beta1.BucketClaim{}).
		Complete(r)
}

func createReconciler(mgr ctrl.Manager, name string, recorder event.Recorder, c managed.ExternalConnector, creationGracePeriod time.Duration) *managed.Reconciler {

	return managed.NewReconciler(mgr,
		resource.ManagedKind(miniov1beta1.BucketClaimGroupVersionKind),
		managed.WithExternalConnector(c),
		managed.WithLogger(logging.NewLogrLogger(mgr.GetLogger().WithValues("controller", name))),
		managed.WithRecorder(recorder),
		managed.WithPollInterval(1*time.Minute),
		managed.WithCreationGracePeriod(creationGracePeriod))
}

// SetupWebhook adds a webhook for managed resources.
func SetupWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &miniov1beta1.BucketClaim{}).
		WithValidator(&Validator{
			log: mgr.GetLogger().WithName("webhook").WithName(strings.ToLower(miniov1beta1.BucketClaimKind)),
		}).
		Complete()
}

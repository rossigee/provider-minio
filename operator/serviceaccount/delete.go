package serviceaccount

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (c *serviceAccountClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("deleting service account")

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalDelete{}, errNotServiceAccount
	}

	accessKey := meta.GetExternalName(serviceAccount)
	if accessKey == "" {
		log.V(1).Info("service account has no external name, nothing to delete")
		return managed.ExternalDelete{}, nil
	}

	log.V(1).Info("deleting service account from MinIO", "accessKey", accessKey)

	// Delete the service account
	err := c.ma.DeleteServiceAccount(ctx, accessKey)
	if err != nil {
		if isServiceAccountNotFound(err) {
			log.V(1).Info("service account already deleted", "accessKey", accessKey)
			c.emitDeletionEvent(serviceAccount)
			return managed.ExternalDelete{}, nil
		}
		c.emitDeletionFailureEvent(serviceAccount, err)
		return managed.ExternalDelete{}, err
	}

	log.V(1).Info("service account deleted successfully", "accessKey", accessKey)
	c.emitDeletionEvent(serviceAccount)
	serviceAccount.SetConditions(xpv1.Deleting())

	return managed.ExternalDelete{}, nil
}

func (c *serviceAccountClient) emitDeletionEvent(serviceAccount *miniov1.ServiceAccount) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Service account successfully deleted",
	})
}

func (c *serviceAccountClient) emitDeletionFailureEvent(serviceAccount *miniov1.ServiceAccount, err error) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeWarning,
		Reason:  "DeleteFailed",
		Message: err.Error(),
	})
}

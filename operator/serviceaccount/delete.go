package serviceaccount

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (s *serviceAccountClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("deleting resource")

	serviceAccount, ok := mg.(*miniov1beta1.ServiceAccount)
	if !ok {
		return managed.ExternalDelete{}, errNotServiceAccount
	}

	accessKey := serviceAccount.Status.AtProvider.AccessKey
	if accessKey == "" {
		accessKey = serviceAccount.GetAccessKey()
	}

	// Check if the service account exists before attempting deletion
	exists, err := s.serviceAccountExists(ctx, accessKey)
	if err != nil {
		// If we can't determine if it exists, log the error but continue
		log.V(1).Info("error checking service account existence during deletion", "error", err)
	}

	if !exists {
		// Service account doesn't exist, consider deletion successful
		log.V(1).Info("service account doesn't exist, deletion successful", "accessKey", accessKey)
		s.emitDeleteEvent(serviceAccount)
		return managed.ExternalDelete{}, nil
	}

	// Delete the service account
	err = s.ma.DeleteServiceAccount(ctx, accessKey)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	s.emitDeleteEvent(serviceAccount)

	return managed.ExternalDelete{}, nil
}

func (s *serviceAccountClient) emitDeleteEvent(serviceAccount *miniov1beta1.ServiceAccount) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Service Account successfully deleted",
	})
}

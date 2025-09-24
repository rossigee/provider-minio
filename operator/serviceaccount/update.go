package serviceaccount

import (
	"context"
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v3"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (s *serviceAccountClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("updating resource")

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalUpdate{}, errNotServiceAccount
	}

	accessKey := serviceAccount.Status.AtProvider.AccessKey
	if accessKey == "" {
		accessKey = serviceAccount.GetAccessKey()
	}

	// Prepare the update request
	req := madmin.UpdateServiceAccountReq{}

	// Update policy if specified
	if serviceAccount.Spec.ForProvider.Policy != "" {
		req.NewPolicy = json.RawMessage(serviceAccount.Spec.ForProvider.Policy)
	}

	// Update name if specified
	if serviceAccount.Spec.ForProvider.Name != "" {
		req.NewName = serviceAccount.Spec.ForProvider.Name
	}

	// Update description if specified
	if serviceAccount.Spec.ForProvider.Description != "" {
		req.NewDescription = serviceAccount.Spec.ForProvider.Description
	}

	// Update expiration if specified
	if serviceAccount.Spec.ForProvider.Expiration != nil {
		req.NewExpiration = &serviceAccount.Spec.ForProvider.Expiration.Time
	}

	// Update secret key if specified (typically not recommended in production)
	if serviceAccount.Spec.ForProvider.SecretKey != "" {
		req.NewSecretKey = serviceAccount.Spec.ForProvider.SecretKey
	}

	// Perform the update
	err := s.ma.UpdateServiceAccount(ctx, accessKey, req)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	s.emitUpdateEvent(serviceAccount)

	return managed.ExternalUpdate{}, nil
}

func (s *serviceAccountClient) emitUpdateEvent(serviceAccount *miniov1.ServiceAccount) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Updated",
		Message: "Service Account successfully updated",
	})
}

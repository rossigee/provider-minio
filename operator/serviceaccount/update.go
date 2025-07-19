package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v4"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (c *serviceAccountClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("updating service account")

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalUpdate{}, errNotServiceAccount
	}

	accessKey := meta.GetExternalName(serviceAccount)
	if accessKey == "" {
		return managed.ExternalUpdate{}, fmt.Errorf("service account has no external name")
	}

	// Prepare update request
	opts := madmin.UpdateServiceAccountReq{}

	// Update description if provided
	if serviceAccount.Spec.ForProvider.Description != "" {
		opts.NewDescription = serviceAccount.Spec.ForProvider.Description
	}

	// Update expiry if provided
	if serviceAccount.Spec.ForProvider.Expiry != nil {
		opts.NewExpiration = &serviceAccount.Spec.ForProvider.Expiry.Time
	}

	// Update additional policies if provided (convert to JSON)
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		policyBytes, err := json.Marshal(serviceAccount.Spec.ForProvider.Policies)
		if err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("failed to marshal policies: %w", err)
		}
		opts.NewPolicy = policyBytes
	}

	log.V(1).Info("updating service account in MinIO", "accessKey", accessKey)

	// Update the service account
	err := c.ma.UpdateServiceAccount(ctx, accessKey, opts)
	if err != nil {
		c.emitUpdateFailureEvent(serviceAccount, err)
		return managed.ExternalUpdate{}, fmt.Errorf("failed to update service account: %w", err)
	}

	log.V(1).Info("service account updated successfully", "accessKey", accessKey)
	c.emitUpdateEvent(serviceAccount)

	return managed.ExternalUpdate{}, nil
}

func (c *serviceAccountClient) emitUpdateEvent(serviceAccount *miniov1.ServiceAccount) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Updated",
		Message: "Service account updated successfully",
	})
}

func (c *serviceAccountClient) emitUpdateFailureEvent(serviceAccount *miniov1.ServiceAccount, err error) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeWarning,
		Reason:  "UpdateFailed",
		Message: err.Error(),
	})
}

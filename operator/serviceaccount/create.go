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

const (
	// ServiceAccountCreatedAnnotationKey is the annotation name where we store the information that the
	// service account has been created.
	ServiceAccountCreatedAnnotationKey string = "minio.crossplane.io/serviceaccount-created"
)

func (c *serviceAccountClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("creating service account")

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalCreation{}, errNotServiceAccount
	}

	// Check if service account is already created by looking for annotation
	if serviceAccount.GetAnnotations()[ServiceAccountCreatedAnnotationKey] == "true" {
		log.V(1).Info("service account already marked as created, skipping creation")
		return managed.ExternalCreation{}, nil
	}

	parentUser := serviceAccount.GetParentUser()
	if parentUser == "" {
		return managed.ExternalCreation{}, fmt.Errorf("parentUser is required for service account creation")
	}

	// Prepare service account creation request
	opts := madmin.AddServiceAccountReq{
		TargetUser: parentUser,
		Name:       serviceAccount.GetServiceAccountName(),
	}

	// Set description if provided
	if serviceAccount.Spec.ForProvider.Description != "" {
		opts.Description = serviceAccount.Spec.ForProvider.Description
	}

	// Set expiry if provided
	if serviceAccount.Spec.ForProvider.Expiry != nil {
		opts.Expiration = &serviceAccount.Spec.ForProvider.Expiry.Time
	}

	// Set additional policies if provided (convert to JSON)
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		policyBytes, err := json.Marshal(serviceAccount.Spec.ForProvider.Policies)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("failed to marshal policies: %w", err)
		}
		opts.Policy = policyBytes
	}

	log.V(1).Info("creating service account in MinIO", "parentUser", parentUser, "name", opts.Name)

	// Create the service account
	creds, err := c.ma.AddServiceAccount(ctx, opts)
	if err != nil {
		c.emitCreationFailureEvent(serviceAccount, err)
		return managed.ExternalCreation{}, fmt.Errorf("failed to create service account: %w", err)
	}

	log.V(1).Info("service account created successfully", "accessKey", creds.AccessKey)

	// Store the access key as the external name
	meta.SetExternalName(serviceAccount, creds.AccessKey)

	// Mark the service account as created
	if serviceAccount.GetAnnotations() == nil {
		serviceAccount.SetAnnotations(map[string]string{})
	}
	annotations := serviceAccount.GetAnnotations()
	annotations[ServiceAccountCreatedAnnotationKey] = "true"
	serviceAccount.SetAnnotations(annotations)

	c.emitCreationEvent(serviceAccount)

	// Return connection details containing the credentials
	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"accessKey":  []byte(creds.AccessKey),
			"secretKey":  []byte(creds.SecretKey),
			"parentUser": []byte(parentUser),
		},
	}, nil
}

func (c *serviceAccountClient) emitCreationEvent(serviceAccount *miniov1.ServiceAccount) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Service account created successfully",
	})
}

func (c *serviceAccountClient) emitCreationFailureEvent(serviceAccount *miniov1.ServiceAccount, err error) {
	c.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeWarning,
		Reason:  "CreateFailed",
		Message: err.Error(),
	})
}

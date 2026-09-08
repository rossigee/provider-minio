package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/madmin-go/v3"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	"github.com/sethvargo/go-password/password"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ServiceAccountCreatedAnnotationKey is the annotation name where we store the information that the
	// service account has been created.
	ServiceAccountCreatedAnnotationKey string = "minio.crossplane.io/serviceaccount-created"
)

func (s *serviceAccountClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {

	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("creating resource")

	serviceAccount, ok := mg.(*miniov1beta1.ServiceAccount)
	if !ok {
		return managed.ExternalCreation{}, errNotServiceAccount
	}

	// Get access key from spec or empty (MinIO will generate one)
	accessKey := serviceAccount.Spec.ForProvider.AccessKey
	secretKey := serviceAccount.Spec.ForProvider.SecretKey
	name := serviceAccount.Spec.ForProvider.Name

	// If CredentialsSecretRef is set, resolve it
	if serviceAccount.Spec.ForProvider.CredentialsSecretRef != nil {
		resolvedAccessKey, resolvedSecretKey, err := minioutil.ResolveCredentialsSecret(ctx, s.kube, serviceAccount.GetNamespace(), serviceAccount.Spec.ForProvider.CredentialsSecretRef)
		if err != nil {
			s.recorder.Event(serviceAccount, event.Event{
				Type:    event.TypeWarning,
				Reason:  "CannotResolveCredentials",
				Message: fmt.Sprintf("Failed to resolve credentials secret: %s", err),
			})
			return managed.ExternalCreation{}, err
		}
		accessKey = resolvedAccessKey
		secretKey = resolvedSecretKey
	}

	// Validate mutually exclusive policy fields
	if serviceAccount.Spec.ForProvider.Policy != "" && len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		return managed.ExternalCreation{}, fmt.Errorf("only one of policy or policies may be specified")
	}

	// For adoption: check if service account already exists by AccessKey or Name
	adoptionKey := accessKey
	if adoptionKey == "" && name != "" {
		adoptionKey = name
	}

	if adoptionKey != "" {
		exists, err := s.serviceAccountExists(ctx, adoptionKey)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
		if exists {
			// Service account already exists - adopt it
			s.emitAdoptionEvent(serviceAccount, adoptionKey)
			// Set the external-name annotation to the existing access key
			meta.SetExternalName(serviceAccount, adoptionKey)
			// Attach policies if specified (for adopted accounts)
			if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
				if err := s.attachPolicies(ctx, adoptionKey, serviceAccount.Spec.ForProvider.Policies); err != nil {
					return managed.ExternalCreation{}, err
				}
			}
			return managed.ExternalCreation{}, nil
		}
	}

	// If no access key is provided but secret key is, that's an error
	// If no access key is provided, don't provide secret key either - let MinIO generate both
	if accessKey == "" && secretKey != "" {
		return managed.ExternalCreation{}, fmt.Errorf("access key must be specified if secret key is specified")
	}

	// Only generate secret key if access key is also provided
	// If neither is provided, let MinIO generate both
	if secretKey == "" && accessKey != "" {
		var err error
		secretKey, err = password.Generate(64, 5, 0, false, true)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	// Prepare the AddServiceAccountReq
	req := madmin.AddServiceAccountReq{
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		TargetUser:  serviceAccount.Spec.ForProvider.TargetUser,
		Name:        name,
		Description: serviceAccount.Spec.ForProvider.Description,
	}

	// Add policy if specified
	if serviceAccount.Spec.ForProvider.Policy != "" {
		req.Policy = json.RawMessage(serviceAccount.Spec.ForProvider.Policy)
	}

	// Add expiration if specified
	if serviceAccount.Spec.ForProvider.Expiration != nil {
		req.Expiration = &serviceAccount.Spec.ForProvider.Expiration.Time
	}

	// Create the service account
	credentials, err := s.ma.AddServiceAccount(ctx, req)
	if err != nil {
		s.recorder.Event(serviceAccount, event.Event{
			Type:    event.TypeWarning,
			Reason:  "CannotCreateExternalResource",
			Message: fmt.Sprintf("Failed to create service account: %s", err),
		})
		return managed.ExternalCreation{}, err
	}

	s.emitCreationEvent(serviceAccount)

	// Set the external-name annotation to the MinIO-generated access key.
	// This is the source of truth for locating the resource in future reconciles.
	meta.SetExternalName(serviceAccount, credentials.AccessKey)

	// Update the status with the created access key (for display/status purposes only)
	serviceAccount.Status.AtProvider.AccessKey = credentials.AccessKey

	// Attach policies if specified
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		if err := s.attachPolicies(ctx, credentials.AccessKey, serviceAccount.Spec.ForProvider.Policies); err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	connectionDetails := managed.ConnectionDetails{
		AccessKeyName: []byte(credentials.AccessKey),
		SecretKeyName: []byte(credentials.SecretKey),
	}

	return managed.ExternalCreation{ConnectionDetails: connectionDetails}, nil
}

func (s *serviceAccountClient) serviceAccountExists(ctx context.Context, accessKey string) (bool, error) {
	// Try to get info about the service account
	_, err := s.ma.InfoServiceAccount(ctx, accessKey)
	if err != nil {
		// Distinguish not-found from transient errors
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		// Transient error (auth, network, etc.) - propagate it to trigger a requeue
		return false, err
	}
	return true, nil
}

func (s *serviceAccountClient) emitCreationEvent(serviceAccount *miniov1beta1.ServiceAccount) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Service Account successfully created",
	})
}

func (s *serviceAccountClient) emitAdoptionEvent(serviceAccount *miniov1beta1.ServiceAccount, accessKey string) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Adopted",
		Message: fmt.Sprintf("Adopted existing service account: %s", accessKey),
	})
}

func (s *serviceAccountClient) attachPolicies(ctx context.Context, accessKey string, policies []string) error {
	for _, policy := range policies {
		_, err := s.ma.AttachPolicy(ctx, madmin.PolicyAssociationReq{
			Policies: []string{policy},
			User:     accessKey,
		})
		if err != nil {
			s.recorder.Event(nil, event.Event{
				Type:    event.TypeWarning,
				Reason:  "CannotAttachPolicy",
				Message: fmt.Sprintf("Failed to attach policy %s to %s: %s", policy, accessKey, err),
			})
			return err
		}
	}
	return nil
}

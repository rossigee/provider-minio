package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v3"
	"github.com/sethvargo/go-password/password"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
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

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalCreation{}, errNotServiceAccount
	}

	// Generate secret key if not provided
	secretKey := serviceAccount.Spec.ForProvider.SecretKey
	if secretKey == "" {
		var err error
		secretKey, err = password.Generate(64, 5, 0, false, true)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	// Prepare the AddServiceAccountReq
	req := madmin.AddServiceAccountReq{
		AccessKey:   serviceAccount.Spec.ForProvider.AccessKey,
		SecretKey:   secretKey,
		TargetUser:  serviceAccount.Spec.ForProvider.TargetUser,
		Name:        serviceAccount.Spec.ForProvider.Name,
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

	// Check if service account already exists
	accessKey := serviceAccount.GetAccessKey()
	if req.AccessKey != "" {
		accessKey = req.AccessKey
	}

	exists, err := s.serviceAccountExists(ctx, accessKey)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if exists {
		return managed.ExternalCreation{}, fmt.Errorf("service account already exists")
	}

	// Create the service account
	credentials, err := s.ma.AddServiceAccount(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	s.emitCreationEvent(serviceAccount)

	annotations := serviceAccount.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[ServiceAccountCreatedAnnotationKey] = "true"
	serviceAccount.SetAnnotations(annotations)

	// Update the status with the created access key
	serviceAccount.Status.AtProvider.AccessKey = credentials.AccessKey

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
		// If we get an error, assume it doesn't exist
		// This might need refinement based on specific error types
		return false, nil
	}
	return true, nil
}

func (s *serviceAccountClient) emitCreationEvent(serviceAccount *miniov1.ServiceAccount) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Service Account successfully created",
	})
}

package serviceaccount

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v3"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
	k8svi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	AccessKeyName = "AWS_ACCESS_KEY_ID"
	SecretKeyName = "AWS_SECRET_ACCESS_KEY"
)

func (s *serviceAccountClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := ctrl.LoggerFrom(ctx)

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalObservation{}, errNotServiceAccount
	}

	// Check if the service account has been created yet
	_, ok = serviceAccount.GetAnnotations()[ServiceAccountCreatedAnnotationKey]
	if !ok && serviceAccount.Status.AtProvider.AccessKey == "" {
		// The service account has not yet been created
		return managed.ExternalObservation{}, nil
	}

	accessKey := serviceAccount.GetAccessKey()
	if serviceAccount.Status.AtProvider.AccessKey != "" {
		// Use the access key from status if available (post-creation)
		accessKey = serviceAccount.Status.AtProvider.AccessKey
	}

	// Check if the service account exists in MinIO
	info, err := s.ma.InfoServiceAccount(ctx, accessKey)
	if err != nil {
		// If we get an error, the service account likely doesn't exist
		log.V(1).Info("service account doesn't exist", "accessKey", accessKey, "error", err)
		serviceAccount.Status.AtProvider.AccessKey = ""
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Update the status with information from MinIO
	serviceAccount.Status.AtProvider.AccessKey = accessKey
	serviceAccount.Status.AtProvider.AccountStatus = info.AccountStatus
	serviceAccount.Status.AtProvider.ParentUser = info.ParentUser
	serviceAccount.Status.AtProvider.ImpliedPolicy = info.ImpliedPolicy
	serviceAccount.Status.AtProvider.Policy = info.Policy

	if info.Expiration != nil {
		serviceAccount.Status.AtProvider.Expiration = &metav1.Time{Time: *info.Expiration}
	}

	// Check if the service account needs to be updated
	if !s.isUpToDate(serviceAccount, info) {
		serviceAccount.SetConditions(miniov1.Updating())
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}

	// Set the condition based on account status
	if info.AccountStatus == "enabled" {
		serviceAccount.SetConditions(xpv1.Available())
	} else {
		serviceAccount.SetConditions(miniov1.Disabled())
	}

	// Validate connection credentials if the service account is not being deleted
	if mg.GetDeletionTimestamp() == nil && mg.GetWriteConnectionSecretToReference() != nil {
		secret := k8svi.Secret{}

		err = s.kube.Get(ctx, types.NamespacedName{
			Namespace: mg.GetWriteConnectionSecretToReference().Namespace,
			Name:      mg.GetWriteConnectionSecretToReference().Name,
		}, &secret)
		if err != nil {
			log.V(1).Info("connection secret not found or not accessible", "error", err)
			// This is not necessarily an error condition during initial creation
		} else {
			log.V(1).Info("service account credentials validated", "accessKey", accessKey)
		}
	}

	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

// isUpToDate checks if the service account configuration matches what's in MinIO
func (s *serviceAccountClient) isUpToDate(serviceAccount *miniov1.ServiceAccount, info madmin.InfoServiceAccountResp) bool {
	// Check if policy needs updating
	if serviceAccount.Spec.ForProvider.Policy != "" && serviceAccount.Spec.ForProvider.Policy != info.Policy {
		return false
	}

	// Check expiration
	specExpiration := serviceAccount.Spec.ForProvider.Expiration
	infoExpiration := info.Expiration

	if (specExpiration == nil) != (infoExpiration == nil) {
		return false
	}

	if specExpiration != nil && infoExpiration != nil {
		if !specExpiration.Time.Equal(*infoExpiration) {
			return false
		}
	}

	// All checks passed
	return true
}

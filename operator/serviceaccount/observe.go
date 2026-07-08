package serviceaccount

import (
	"context"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
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

	serviceAccount, ok := mg.(*miniov1beta1.ServiceAccount)
	if !ok {
		return managed.ExternalObservation{}, errNotServiceAccount
	}

	// Get the external-name (MinIO access key) - source of truth for resource identity.
	// This is set during Create() and persisted via crossplane-runtime's
	// UpdateCriticalAnnotations mechanism, which survives status-write failures.
	accessKey := meta.GetExternalName(serviceAccount)
	if accessKey == "" {
		// Resource has not yet been created (no external-name set)
		return managed.ExternalObservation{}, nil
	}

	// Check if the service account exists in MinIO
	info, err := s.ma.InfoServiceAccount(ctx, accessKey)
	if err != nil {
		// Distinguish not-found from transient errors
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			log.V(1).Info("service account doesn't exist", "accessKey", accessKey)
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		// Transient error (auth, network, etc.) - let the reconciler handle it with a requeue
		log.V(1).Info("error checking service account existence", "accessKey", accessKey, "error", err)
		return managed.ExternalObservation{}, err
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
		serviceAccount.SetConditions(miniov1beta1.Updating())
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}

	// Set the condition based on account status
	if info.AccountStatus == "enabled" {
		serviceAccount.SetConditions(xpv1.Available())
	} else {
		serviceAccount.SetConditions(miniov1beta1.Disabled())
	}

	// Validate connection credentials if the service account is not being deleted
	if mg.GetDeletionTimestamp() == nil && mg.(resource.ModernManaged).GetWriteConnectionSecretToReference() != nil {
		secret := k8svi.Secret{}

		err = s.kube.Get(ctx, types.NamespacedName{
			Namespace: mg.GetNamespace(),
			Name:      mg.(resource.ModernManaged).GetWriteConnectionSecretToReference().Name,
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
func (s *serviceAccountClient) isUpToDate(serviceAccount *miniov1beta1.ServiceAccount, info madmin.InfoServiceAccountResp) bool {
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

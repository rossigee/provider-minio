package serviceaccount

import (
	"context"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v4"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (c *serviceAccountClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("observing service account")

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return managed.ExternalObservation{}, errNotServiceAccount
	}

	// Get the external name (access key) - if empty, resource doesn't exist yet
	accessKey := meta.GetExternalName(serviceAccount)
	if accessKey == "" {
		log.V(1).Info("service account has no external name, does not exist yet")
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Check if service account exists in MinIO
	info, err := c.ma.InfoServiceAccount(ctx, accessKey)
	if err != nil {
		if isServiceAccountNotFound(err) {
			log.V(1).Info("service account not found in MinIO", "accessKey", accessKey)
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("failed to get service account info: %w", err)
	}

	log.V(1).Info("service account found in MinIO", "accessKey", accessKey, "parentUser", info.ParentUser)

	// Update the status with current information
	serviceAccount.Status.AtProvider.AccessKey = accessKey
	serviceAccount.Status.AtProvider.ParentUser = info.ParentUser
	serviceAccount.Status.AtProvider.Status = "enabled" // MinIO service accounts are enabled by default

	// Convert policy to string for status
	if info.Policy != "" {
		serviceAccount.Status.AtProvider.Policies = info.Policy
	}

	// Check if the service account is up to date
	upToDate := c.isUpToDate(serviceAccount, info)

	// Set the ready condition
	serviceAccount.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
		ConnectionDetails: managed.ConnectionDetails{
			"accessKey":  []byte(accessKey),
			"parentUser": []byte(info.ParentUser),
		},
	}, nil
}

// isUpToDate checks if the service account matches the desired configuration
func (c *serviceAccountClient) isUpToDate(serviceAccount *miniov1.ServiceAccount, info madmin.InfoServiceAccountResp) bool {
	// Check if parent user matches
	if serviceAccount.GetParentUser() != info.ParentUser {
		return false
	}

	// Check if policies match (if specified)
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		// Convert our slice to string for comparison with info.Policy
		desiredPolicies := strings.Join(serviceAccount.Spec.ForProvider.Policies, ",")
		if info.Policy != desiredPolicies {
			return false
		}
	}

	return true
}

// isServiceAccountNotFound checks if the error indicates the service account was not found
func isServiceAccountNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "NoSuchServiceAccount")
}

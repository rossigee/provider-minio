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
	ctrl "sigs.k8s.io/controller-runtime"
)

func (s *serviceAccountClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("updating resource")

	serviceAccount, ok := mg.(*miniov1beta1.ServiceAccount)
	if !ok {
		return managed.ExternalUpdate{}, errNotServiceAccount
	}

	// Get the external-name (MinIO access key) for this resource
	accessKey := meta.GetExternalName(serviceAccount)
	if accessKey == "" {
		return managed.ExternalUpdate{}, fmt.Errorf("service account has not been created yet (no external-name)")
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

	// Perform the update if any inline fields changed
	if req.NewPolicy != nil || req.NewName != "" || req.NewDescription != "" || req.NewExpiration != nil || req.NewSecretKey != "" {
		err := s.ma.UpdateServiceAccount(ctx, accessKey, req)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	// Handle named policy attachments
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		// Get current policies
		userInfo, err := s.ma.GetUserInfo(ctx, accessKey)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
		currentPolicies := []string{}
		if userInfo.PolicyName != "" {
			for _, p := range strings.Split(userInfo.PolicyName, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					currentPolicies = append(currentPolicies, p)
				}
			}
		}
		// Detach policies not in desired
		for _, current := range currentPolicies {
			needDetach := true
			for _, desired := range serviceAccount.Spec.ForProvider.Policies {
				if current == desired {
					needDetach = false
					break
				}
			}
			if needDetach {
				_, err := s.ma.DetachPolicy(ctx, madmin.PolicyAssociationReq{
					Policies: []string{current},
					User:     accessKey,
				})
				if err != nil {
					return managed.ExternalUpdate{}, err
				}
			}
		}
		// Attach desired policies not already attached
		for _, desired := range serviceAccount.Spec.ForProvider.Policies {
			found := false
			for _, current := range currentPolicies {
				if desired == current {
					found = true
					break
				}
			}
			if !found {
				_, err := s.ma.AttachPolicy(ctx, madmin.PolicyAssociationReq{
					Policies: []string{desired},
					User:     accessKey,
				})
				if err != nil {
					return managed.ExternalUpdate{}, err
				}
			}
		}
	}

	s.emitUpdateEvent(serviceAccount)

	return managed.ExternalUpdate{}, nil
}

func (s *serviceAccountClient) emitUpdateEvent(serviceAccount *miniov1beta1.ServiceAccount) {
	s.recorder.Event(serviceAccount, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Updated",
		Message: "Service Account successfully updated",
	})
}

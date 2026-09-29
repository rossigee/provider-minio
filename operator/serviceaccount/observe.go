package serviceaccount

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
		// If external-name not set, check if AccessKey is specified in spec (adoption case)
		if serviceAccount.Spec.ForProvider.AccessKey != "" {
			accessKey = serviceAccount.Spec.ForProvider.AccessKey
			// Fall through to check if this AccessKey exists in MinIO for adoption
		} else if serviceAccount.Spec.ForProvider.Name != "" {
			// Also try the Name field as a fallback for adoption
			accessKey = serviceAccount.Spec.ForProvider.Name
		} else {
			// Resource has not yet been created (no external-name or AccessKey/Name set)
			return managed.ExternalObservation{}, nil
		}
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

	// Check if named policies need to be attached
	if len(serviceAccount.Spec.ForProvider.Policies) > 0 {
		userInfo, err := s.ma.GetUserInfo(ctx, accessKey)
		if err != nil {
			// If GetUserInfo fails (e.g., IAM not allowed for service accounts), assume up-to-date
			// to avoid update loop. The policy was attached at creation.
			if strings.Contains(err.Error(), "IAM action is not allowed") || strings.Contains(err.Error(), "does not exist") {
				log.V(1).Info("cannot get user info for policy check, assuming up-to-date", "accessKey", accessKey, "error", err)
			} else {
				log.V(1).Info("cannot get user info for policy check", "accessKey", accessKey, "error", err)
				serviceAccount.SetConditions(miniov1beta1.Updating())
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
			}
		} else {
			// PolicyName is comma-separated list for multiple policies (or single)
			currentPolicies := []string{}
			if userInfo.PolicyName != "" {
				// MinIO returns comma-separated policies
				for _, p := range strings.Split(userInfo.PolicyName, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						currentPolicies = append(currentPolicies, p)
					}
				}
			}
			desiredPolicies := serviceAccount.Spec.ForProvider.Policies
			policiesMatch := len(desiredPolicies) == len(currentPolicies)
			if policiesMatch {
				for _, desired := range desiredPolicies {
					found := false
					for _, current := range currentPolicies {
						if desired == current {
							found = true
							break
						}
					}
					if !found {
						policiesMatch = false
						break
					}
				}
			}
			if !policiesMatch {
				serviceAccount.SetConditions(miniov1beta1.Updating())
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
			}
		}
	}

	// Set the condition based on account status.
	//
	// MinIO is inconsistent here: for users the admin API reports the account
	// status as madmin.AccountEnabled ("enabled"), but for service accounts
	// InfoServiceAccountResp reports "on". Comparing only against "enabled"
	// therefore never matched a service account and every one was reported
	// Disabled. Accept either spelling.
	if accountStatusEnabled(info.AccountStatus) {
		serviceAccount.SetConditions(xpv1.Available())
	} else {
		serviceAccount.SetConditions(miniov1beta1.Disabled())
	}

	// Validate connection credentials if the service account is not being deleted
	if mg.GetDeletionTimestamp() == nil && mg.(resource.ModernManaged).GetWriteConnectionSecretToReference() != nil {
		secret := corev1.Secret{}

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
	// Check if inline policy needs updating.
	//
	// This must compare the policy as JSON, not as a raw string. MinIO
	// re-serialises the policy it stores, so the bytes it returns never match the
	// bytes in the spec even when the documents are identical. A string
	// comparison therefore reported a difference on every single reconcile, so
	// the controller issued an UpdateServiceAccount call each time and the
	// resource never converged to UpToDate - it sat at Updating forever,
	// hammering MinIO once per poll interval.
	if serviceAccount.Spec.ForProvider.Policy != "" && !policiesEqual(serviceAccount.Spec.ForProvider.Policy, info.Policy) {
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

	// Note: Policies (named policy attachments) are checked in Observe via GetUserInfo
	// to avoid needing context in this helper. If Policies are specified, we consider
	// inline check passed and rely on Observe to verify attachment.
	return true
}

// policiesEqual reports whether two MinIO policy documents are equivalent.
//
// MinIO re-serialises the policy it stores, so a raw string compare never matches.
// It also normalises array order: a spec declaring
// Action: ["s3:GetObject", "s3:PutObject", "s3:ListBucket"] comes back as
// ["s3:GetObject", "s3:ListBucket", "s3:PutObject"]. Action, Resource and
// Condition are sets in an IAM policy, so order carries no meaning and is
// canonicalised away here.
//
// Comparing raw strings made the controller report a difference on every
// reconcile, so it called UpdateServiceAccount each time and the resource never
// converged to UpToDate - it sat at Updating forever, hammering MinIO once per
// poll interval.
func policiesEqual(a, b string) bool {
	if a == b {
		return true
	}

	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		return false
	}
	return canonicalJSON(av) == canonicalJSON(bv)
}

// canonicalJSON renders a decoded JSON value with object keys sorted and array
// elements sorted by their own canonical form, so that two documents differing
// only in key or element order serialise identically.
func canonicalJSON(v any) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, strconv.Quote(k)+":"+canonicalJSON(t[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, canonicalJSON(e))
		}
		sort.Strings(parts)
		return "[" + strings.Join(parts, ",") + "]"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// accountStatusEnabled reports whether a MinIO account status means enabled.
// madmin.AccountEnabled is "enabled", which is what the admin API reports for
// users, but InfoServiceAccountResp reports "on" for service accounts. Both are
// accepted so a service account is not permanently reported Disabled.
func accountStatusEnabled(status string) bool {
	return status == string(madmin.AccountEnabled) || status == "on"
}

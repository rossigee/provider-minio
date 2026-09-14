package policy

import (
	"context"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (p *policyClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("deleting resource")

	policy, ok := mg.(*miniov1beta1.Policy)
	if !ok {
		return managed.ExternalDelete{}, errNotPolicy
	}

	policy.SetConditions(xpv1.Deleting())

	policyName := policy.GetName()
	err := p.ma.RemoveCannedPolicy(ctx, policyName)
	if err != nil {
		// Check if this is a reserved/inbuilt policy that can never be deleted
		if isReservedPolicyError(err) {
			log.V(1).Info("policy is reserved and cannot be deleted", "policy", policyName)
			// Treat as terminal - strip finalizer to allow deletion
			policy.SetFinalizers([]string{})
			p.emitReservedPolicyEvent(policy)
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, err
	}

	p.emitDeletionEvent(policy)
	return managed.ExternalDelete{}, nil
}

func (p *policyClient) emitDeletionEvent(policy *miniov1beta1.Policy) {
	p.recorder.Event(policy, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Policy successfully deleted",
	})
}

func (p *policyClient) emitReservedPolicyEvent(policy *miniov1beta1.Policy) {
	p.recorder.Event(policy, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Policy is reserved and cannot be deleted by this provider - allowing Kubernetes deletion",
	})
}

func isReservedPolicyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "inbuilt policy") && strings.Contains(msg, "not allowed to be deleted")
}

package policy

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotPolicy = fmt.Errorf("managed resource is not a policy")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.ModernTracker

	// newAdmin creates the MinIO admin client. It is a field so tests can
	// substitute a fake; when nil the real client is used.
	newAdmin adminFactory
}

type adminFactory func(context.Context, client.Client, *providerv1beta1.ProviderConfig) (minioutil.PolicyAdmin, error)

func newRealAdmin(ctx context.Context, kube client.Client, config *providerv1beta1.ProviderConfig) (minioutil.PolicyAdmin, error) {
	return minioutil.NewMinioAdmin(ctx, kube, config)
}

type policyClient struct {
	ma       minioutil.PolicyAdmin
	recorder event.Recorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg.(resource.ModernManaged))
	if err != nil {
		return nil, err
	}

	policy, ok := mg.(*miniov1beta1.Policy)
	if !ok {
		return nil, errNotPolicy
	}

	config, err := c.getProviderConfig(ctx, policy)
	if err != nil {
		return nil, err
	}

	maker := c.newAdmin
	if maker == nil {
		maker = newRealAdmin
	}

	ma, err := maker(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	uc := &policyClient{
		ma:       ma,
		recorder: c.recorder,
	}

	return uc, nil
}

func (c *connector) getProviderConfig(ctx context.Context, policy *miniov1beta1.Policy) (*providerv1beta1.ProviderConfig, error) {
	configName := policy.GetProviderConfigReference().Name
	config := &providerv1beta1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

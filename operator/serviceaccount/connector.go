package serviceaccount

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v3"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotServiceAccount = fmt.Errorf("managed resource is not a service account")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.Tracker
}

type serviceAccountClient struct {
	ma          *madmin.AdminClient
	kube        client.Client
	recorder    event.Recorder
	url         *url.URL
	tlsSettings bool
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg)
	if err != nil {
		return nil, err
	}

	serviceAccount, ok := mg.(*miniov1.ServiceAccount)
	if !ok {
		return nil, errNotServiceAccount
	}

	config, err := c.getProviderConfig(ctx, serviceAccount)
	if err != nil {
		return nil, err
	}

	ma, err := minioutil.NewMinioAdmin(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Spec.MinioURL)
	if err != nil {
		return nil, err
	}

	sac := &serviceAccountClient{
		ma:          ma,
		kube:        c.kube,
		recorder:    c.recorder,
		url:         parsed,
		tlsSettings: minioutil.IsTLSEnabled(parsed),
	}

	return sac, nil
}

func (c *connector) getProviderConfig(ctx context.Context, serviceAccount *miniov1.ServiceAccount) (*providerv1.ProviderConfig, error) {
	configName := serviceAccount.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

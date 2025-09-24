package user

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/madmin-go/v3"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotUser = fmt.Errorf("managed resource is not a user")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.Tracker
}

type userClient struct {
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

	var config *providerv1.ProviderConfig

	if userv1, ok := mg.(*miniov1.User); ok {
		config, err = c.getProviderConfigV1(ctx, userv1)
	} else if userv1beta1, ok := mg.(*miniov1beta1.User); ok {
		config, err = c.getProviderConfigV1Beta1(ctx, userv1beta1)
	} else {
		return nil, errNotUser
	}
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

	uc := &userClient{
		ma:          ma,
		kube:        c.kube,
		recorder:    c.recorder,
		url:         parsed,
		tlsSettings: minioutil.IsTLSEnabled(parsed),
	}

	return uc, nil
}

func (c *connector) getProviderConfigV1(ctx context.Context, user *miniov1.User) (*providerv1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

func (c *connector) getProviderConfigV1Beta1(ctx context.Context, user *miniov1beta1.User) (*providerv1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

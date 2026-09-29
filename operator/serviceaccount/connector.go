package serviceaccount

import (
	"context"
	"errors"
	"fmt"
	"net/url"

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
	errNotServiceAccount   = fmt.Errorf("managed resource is not a service account")
	errGetConnectionSecret = errors.New("cannot get connection secret")

	// errConnectionSecretUnrecoverable is returned when a service account exists
	// in MinIO but its connection secret is gone. MinIO stores secret keys
	// hashed and never returns them, so the credentials cannot be reissued
	// without deleting and recreating the service account.
	errConnectionSecretUnrecoverable = errors.New(
		"connection secret is missing and its credentials are unrecoverable: " +
			"MinIO never returns secret keys, so delete this ServiceAccount to have " +
			"a new set issued and written to the connection secret")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.ModernTracker

	// newAdmin is a seam so tests can substitute a fake. When nil the real
	// MinIO admin client is used.
	newAdmin adminFactory
}

type adminFactory func(context.Context, client.Client, *providerv1beta1.ProviderConfig) (minioutil.ServiceAccountAdmin, error)

func newRealAdmin(ctx context.Context, kube client.Client, config *providerv1beta1.ProviderConfig) (minioutil.ServiceAccountAdmin, error) {
	return minioutil.NewMinioAdmin(ctx, kube, config)
}

type serviceAccountClient struct {
	ma          minioutil.ServiceAccountAdmin
	kube        client.Client
	recorder    event.Recorder
	url         *url.URL
	tlsSettings bool
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg.(resource.ModernManaged))
	if err != nil {
		return nil, err
	}

	serviceAccount, ok := mg.(*miniov1beta1.ServiceAccount)
	if !ok {
		return nil, errNotServiceAccount
	}

	config, err := c.getProviderConfig(ctx, serviceAccount)
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

func (c *connector) getProviderConfig(ctx context.Context, serviceAccount *miniov1beta1.ServiceAccount) (*providerv1beta1.ProviderConfig, error) {
	configName := serviceAccount.GetProviderConfigReference().Name
	config := &providerv1beta1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

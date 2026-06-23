package notificationconfiguration

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotNotificationConfiguration = fmt.Errorf("managed resource is not a NotificationConfiguration")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.ModernTracker
}

type notificationClient struct {
	ma       *madmin.AdminClient
	mc       *minio.Client
	kube     client.Client
	cfg      *providerv1.ProviderConfig
	recorder event.Recorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg.(resource.ModernManaged))
	if err != nil {
		return nil, err
	}

	cr, ok := mg.(*miniov1beta1.NotificationConfiguration)
	if !ok {
		return nil, errNotNotificationConfiguration
	}

	config, err := c.getProviderConfig(ctx, cr)
	if err != nil {
		return nil, err
	}

	ma, err := minioutil.NewMinioAdmin(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	mc, err := minioutil.NewMinioClient(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	nc := &notificationClient{
		ma:       ma,
		mc:       mc,
		kube:     c.kube,
		cfg:      config,
		recorder: c.recorder,
	}

	return nc, nil
}

func (c *connector) getProviderConfig(ctx context.Context, cr *miniov1beta1.NotificationConfiguration) (*providerv1.ProviderConfig, error) {
	configName := cr.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

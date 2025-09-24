package bucket

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	minio "github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ managed.ExternalConnecter = &connector{}
var _ managed.ExternalClient = &bucketClient{}

const lockAnnotation = miniov1.Group + "/lock"

var (
	errNotBucket = fmt.Errorf("managed resource is not a bucket")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.Tracker
}

type bucketClient struct {
	mc       *minio.Client
	recorder event.Recorder
}

// Connect implements managed.ExternalConnecter.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg)
	if err != nil {
		return nil, err
	}

	var config *providerv1.ProviderConfig

	// Handle both v1 and v1beta1 API versions
	if bucketv1, ok := mg.(*miniov1.Bucket); ok {
		log.V(1).Info("Connecting v1 bucket", "name", bucketv1.Name)
		config, err = c.getProviderConfigV1(ctx, bucketv1)
		if err != nil {
			return nil, err
		}
	} else if bucketv1beta1, ok := mg.(*miniov1beta1.Bucket); ok {
		log.V(1).Info("Connecting v1beta1 bucket", "name", bucketv1beta1.Name)
		config, err = c.getProviderConfigV1Beta1(ctx, bucketv1beta1)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errNotBucket
	}

	mc, err := minioutil.NewMinioClient(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	bc := &bucketClient{
		mc:       mc,
		recorder: c.recorder,
	}

	return bc, nil
}

func (c *connector) getProviderConfigV1(ctx context.Context, bucket *miniov1.Bucket) (*providerv1.ProviderConfig, error) {
	configName := bucket.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

func (c *connector) getProviderConfigV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket) (*providerv1.ProviderConfig, error) {
	configName := bucket.GetProviderConfigReference().Name
	config := &providerv1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

package bucket

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

var _ managed.ExternalConnector = &connector{}
var _ managed.ExternalClient = &bucketClient{}

const lockAnnotation = miniov1beta1.Group + "/lock"

var (
	errNotBucket = fmt.Errorf("managed resource is not a bucket")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.ModernTracker

	// newS3 is a seam so tests can substitute a fake. When nil the real MinIO
	// S3 client is used.
	newS3 s3Factory
}

type s3Factory func(context.Context, client.Client, *providerv1beta1.ProviderConfig) (minioutil.BucketS3, error)

func newRealS3(ctx context.Context, kube client.Client, config *providerv1beta1.ProviderConfig) (minioutil.BucketS3, error) {
	return minioutil.NewMinioClient(ctx, kube, config)
}

type bucketClient struct {
	mc       minioutil.BucketS3
	recorder event.Recorder
}

// Connect implements managed.ExternalConnector.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg.(resource.ModernManaged))
	if err != nil {
		return nil, err
	}

	var config *providerv1beta1.ProviderConfig

	bucket, ok := mg.(*miniov1beta1.Bucket)
	if !ok {
		return nil, errNotBucket
	}

	log.V(1).Info("Connecting bucket", "name", bucket.Name)
	config, err = c.getProviderConfig(ctx, bucket)
	if err != nil {
		return nil, err
	}

	maker := c.newS3
	if maker == nil {
		maker = newRealS3
	}

	mc, err := maker(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	bc := &bucketClient{
		mc:       mc,
		recorder: c.recorder,
	}

	return bc, nil
}

func (c *connector) getProviderConfig(ctx context.Context, bucket *miniov1beta1.Bucket) (*providerv1beta1.ProviderConfig, error) {
	configName := bucket.GetProviderConfigReference().Name
	config := &providerv1beta1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

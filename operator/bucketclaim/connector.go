package bucketclaim

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	minio "github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"github.com/rossigee/provider-minio/internal/clients"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ managed.ExternalConnecter = &connector{}
var _ managed.ExternalClient = &bucketClaimClient{}

const bucketClaimLockAnnotation = miniov1beta1.Group + "/bucketclaim-lock"

var (
	errNotBucketClaim = fmt.Errorf("managed resource is not a bucket claim")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
}

type bucketClaimClient struct {
	mc       *minio.Client
	recorder event.Recorder
}

// Connect implements managed.ExternalConnecter.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting bucket claim resource")

	bucketClaim, ok := mg.(*miniov1beta1.BucketClaim)
	if !ok {
		return nil, errNotBucketClaim
	}

	log.V(1).Info("Connecting bucket claim", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)

	// Get bucket configuration from the credentials secret
	bucketConfig, err := clients.GetBucketConfig(ctx, c.kube, bucketClaim)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket config: %w", err)
	}

	// Create MinIO client
	mc, err := minio.New(bucketConfig.Endpoint, &minio.Options{
		Creds:  bucketConfig.Credentials,
		Secure: bucketConfig.UseSSL,
		Region: bucketConfig.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	bcc := &bucketClaimClient{
		mc:       mc,
		recorder: c.recorder,
	}

	return bcc, nil
}

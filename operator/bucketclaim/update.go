package bucketclaim

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClaimClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("updating bucket claim resource")

	bucketClaim, ok := mg.(*miniov1beta1.BucketClaim)
	if !ok {
		return managed.ExternalUpdate{}, errNotBucketClaim
	}

	log.V(1).Info("Updating bucket claim", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)

	bucketName := bucketClaim.GetBucketName()

	// Update bucket policy if specified
	if bucketClaim.Spec.Policy != nil {
		err := b.mc.SetBucketPolicy(ctx, bucketName, *bucketClaim.Spec.Policy)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
	} else {
		// Remove policy if not specified
		err := b.mc.SetBucketPolicy(ctx, bucketName, "")
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	return managed.ExternalUpdate{}, nil
}

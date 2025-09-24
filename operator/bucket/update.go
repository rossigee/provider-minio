package bucket

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("updating resource")

	bucket, ok := mg.(*miniov1beta1.Bucket)
	if !ok {
		return managed.ExternalUpdate{}, errNotBucket
	}

	if bucket.Spec.ForProvider.Policy != nil {
		err := b.mc.SetBucketPolicy(ctx, bucket.GetBucketName(), *bucket.Spec.ForProvider.Policy)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	return managed.ExternalUpdate{}, nil
}

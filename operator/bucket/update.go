package bucket

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/minio-go/v7/pkg/tags"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (b *bucketClient) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log := ctrl.LoggerFrom(ctx)
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

	if bucket.Spec.ForProvider.Tags != nil {
		if len(bucket.Spec.ForProvider.Tags) == 0 {
			if err := b.mc.RemoveBucketTagging(ctx, bucket.GetBucketName()); err != nil {
				return managed.ExternalUpdate{}, err
			}
		} else {
			bucketTags, err := tags.NewTags(bucket.Spec.ForProvider.Tags, false)
			if err != nil {
				return managed.ExternalUpdate{}, err
			}
			if err := b.mc.SetBucketTagging(ctx, bucket.GetBucketName(), bucketTags); err != nil {
				return managed.ExternalUpdate{}, err
			}
		}
	}

	return managed.ExternalUpdate{}, nil
}

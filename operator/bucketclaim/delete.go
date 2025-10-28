package bucketclaim

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClaimClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("deleting bucket claim resource")

	bucketClaim, ok := mg.(*miniov1beta1.BucketClaim)
	if !ok {
		return managed.ExternalDelete{}, errNotBucketClaim
	}

	log.V(1).Info("Deleting bucket for claim", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)

	if bucketClaim.Spec.BucketDeletionPolicy == miniov1beta1.DeleteAll {
		err := b.deleteAllObjects(ctx, bucketClaim)
		if err != nil {
			return managed.ExternalDelete{}, err
		}
	}

	err := b.deleteS3Bucket(ctx, bucketClaim)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	return managed.ExternalDelete{}, nil
}

func (b *bucketClaimClient) deleteAllObjects(ctx context.Context, bucketClaim *miniov1beta1.BucketClaim) error {
	log := controllerruntime.LoggerFrom(ctx)
	bucketName := bucketClaim.Status.AtProvider.BucketName

	objectsCh := make(chan minio.ObjectInfo)

	// Send object names that are needed to be removed to objectsCh
	go func() {
		defer close(objectsCh)
		for object := range b.mc.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				log.V(1).Info("warning: cannot list object", "key", object.Key, "error", object.Err)
				continue
			}
			objectsCh <- object
		}
	}()

	// Remove objects
	for rErr := range b.mc.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{}) {
		if rErr.Err != nil {
			return fmt.Errorf("object %q cannot be removed: %w", rErr.ObjectName, rErr.Err)
		}
	}

	return nil
}

func (b *bucketClaimClient) deleteS3Bucket(ctx context.Context, bucketClaim *miniov1beta1.BucketClaim) error {
	bucketName := bucketClaim.Status.AtProvider.BucketName
	err := b.mc.RemoveBucket(ctx, bucketName)
	return err
}

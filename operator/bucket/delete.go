package bucket

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClient) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("deleting resource")

	// Handle both v1 and v1beta1 API versions
	if bucketv1, ok := mg.(*miniov1.Bucket); ok {
		log.V(1).Info("Deleting v1 bucket", "name", bucketv1.Name)
		return b.deleteV1(ctx, bucketv1)
	}

	if bucketv1beta1, ok := mg.(*miniov1beta1.Bucket); ok {
		log.V(1).Info("Deleting v1beta1 bucket", "name", bucketv1beta1.Name)
		return b.deleteV1Beta1(ctx, bucketv1beta1)
	}

	return managed.ExternalDelete{}, errNotBucket
}
func hasDeleteAllPolicy(bucket *miniov1.Bucket) bool {
	return bucket.Spec.ForProvider.BucketDeletionPolicy == miniov1.DeleteAll
}

func (b *bucketClient) deleteAllObjects(ctx context.Context, bucket *miniov1.Bucket) error {
	log := controllerruntime.LoggerFrom(ctx)
	bucketName := bucket.Status.AtProvider.BucketName

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

	bypassGovernance, err := b.isBucketLockEnabled(ctx, bucketName)
	if err != nil {
		log.Error(err, "not able to determine ObjectLock status for bucket", "bucket", bucketName)
	}

	for obj := range b.mc.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{GovernanceBypass: bypassGovernance}) {
		return fmt.Errorf("object %q cannot be removed: %w", obj.ObjectName, obj.Err)
	}
	return nil
}

func (b *bucketClient) isBucketLockEnabled(ctx context.Context, bucketName string) (bool, error) {
	_, mode, _, _, err := b.mc.GetObjectLockConfig(ctx, bucketName)
	if err != nil && err.Error() == "Object Lock configuration does not exist for this bucket" {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// If there's an objectLockConfig it could still be disabled...
	return mode != nil, nil
}

// deleteS3Bucket deletes the bucket.
// NOTE: The removal fails if there are still objects in the bucket.
// This func does not recursively delete all objects beforehand.
func (b *bucketClient) deleteS3Bucket(ctx context.Context, bucket *miniov1.Bucket) error {
	bucketName := bucket.Status.AtProvider.BucketName
	err := b.mc.RemoveBucket(ctx, bucketName)
	return err
}

func (b *bucketClient) emitDeletionEvent(bucket *miniov1.Bucket) {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Bucket deleted",
	})
}

// v1 deletion functions
func (b *bucketClient) deleteV1(ctx context.Context, bucket *miniov1.Bucket) (managed.ExternalDelete, error) {
	if hasDeleteAllPolicy(bucket) {
		err := b.deleteAllObjects(ctx, bucket)
		if err != nil {
			return managed.ExternalDelete{}, err
		}
	}

	err := b.deleteS3Bucket(ctx, bucket)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	b.emitDeletionEvent(bucket)
	return managed.ExternalDelete{}, nil
}

// v1beta1 deletion functions
func (b *bucketClient) deleteV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket) (managed.ExternalDelete, error) {
	if hasDeleteAllPolicyV1Beta1(bucket) {
		err := b.deleteAllObjectsV1Beta1(ctx, bucket)
		if err != nil {
			return managed.ExternalDelete{}, err
		}
	}

	err := b.deleteS3BucketV1Beta1(ctx, bucket)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	b.emitDeletionEventV1Beta1(bucket)
	return managed.ExternalDelete{}, nil
}

func hasDeleteAllPolicyV1Beta1(bucket *miniov1beta1.Bucket) bool {
	return bucket.Spec.ForProvider.BucketDeletionPolicy == miniov1beta1.DeleteAll
}

func (b *bucketClient) deleteAllObjectsV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket) error {
	log := controllerruntime.LoggerFrom(ctx)
	bucketName := bucket.Status.AtProvider.BucketName

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

	bypassGovernance, err := b.isBucketLockEnabled(ctx, bucketName)
	if err != nil {
		log.Error(err, "not able to determine ObjectLock status for bucket", "bucket", bucketName)
	}

	for obj := range b.mc.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{GovernanceBypass: bypassGovernance}) {
		return fmt.Errorf("object %q cannot be removed: %w", obj.ObjectName, obj.Err)
	}
	return nil
}

func (b *bucketClient) deleteS3BucketV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket) error {
	bucketName := bucket.Status.AtProvider.BucketName
	err := b.mc.RemoveBucket(ctx, bucketName)
	return err
}

func (b *bucketClient) emitDeletionEventV1Beta1(bucket *miniov1beta1.Bucket) {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Bucket deleted",
	})
}

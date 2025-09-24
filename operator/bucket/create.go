package bucket

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("creating resource")

	// Handle both v1 and v1beta1 API versions
	if bucketv1, ok := mg.(*miniov1.Bucket); ok {
		log.V(1).Info("Creating v1 bucket", "name", bucketv1.Name)

		err := b.createS3BucketV1(ctx, bucketv1)
		if err != nil {
			return managed.ExternalCreation{}, err
		}

		if bucketv1.Spec.ForProvider.Policy != nil {
			err = b.mc.SetBucketPolicy(ctx, bucketv1.GetBucketName(), *bucketv1.Spec.ForProvider.Policy)
			if err != nil {
				return managed.ExternalCreation{}, err
			}
		}

		b.setLockV1(bucketv1)
		return managed.ExternalCreation{}, b.emitCreationEventV1(bucketv1)

	} else if bucketv1beta1, ok := mg.(*miniov1beta1.Bucket); ok {
		log.V(1).Info("Creating v1beta1 bucket", "name", bucketv1beta1.Name)

		err := b.createS3BucketV1Beta1(ctx, bucketv1beta1)
		if err != nil {
			return managed.ExternalCreation{}, err
		}

		if bucketv1beta1.Spec.ForProvider.Policy != nil {
			err = b.mc.SetBucketPolicy(ctx, bucketv1beta1.GetBucketName(), *bucketv1beta1.Spec.ForProvider.Policy)
			if err != nil {
				return managed.ExternalCreation{}, err
			}
		}

		b.setLockV1Beta1(bucketv1beta1)
		return managed.ExternalCreation{}, b.emitCreationEventV1Beta1(bucketv1beta1)

	} else {
		return managed.ExternalCreation{}, errNotBucket
	}
}

// createS3BucketV1 creates a new bucket and sets the name in the status.
// If the bucket already exists, and we have permissions to access it, no error is returned and the name is set in the status.
// If the bucket exists, but we don't own it, an error is returned.
func (b *bucketClient) createS3BucketV1(ctx context.Context, bucket *miniov1.Bucket) error {
	bucketName := bucket.GetBucketName()
	err := b.mc.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucket.Spec.ForProvider.Region})

	if err != nil {
		// Check to see if we already own this bucket (which happens if we run this twice)
		exists, errBucketExists := b.mc.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			return nil
		}
		// someone else might have created the bucket
		return err

	}
	return nil
}

// setLockV1 sets an annotation that tells the Observe func that we have successfully created the bucket.
// Without it, another resource that has the same bucket name might "adopt" the same bucket, causing 2 resources managing 1 bucket.
func (b *bucketClient) setLockV1(bucket *miniov1.Bucket) {
	if bucket.Annotations == nil {
		bucket.Annotations = map[string]string{}
	}
	bucket.Annotations[lockAnnotation] = "claimed"

}

func (b *bucketClient) emitCreationEventV1(bucket *miniov1.Bucket) error {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Bucket successfully created",
	})
	return nil
}

// createS3BucketV1Beta1 creates a new bucket and sets the name in the status for v1beta1 API.
func (b *bucketClient) createS3BucketV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket) error {
	bucketName := bucket.GetBucketName()
	err := b.mc.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucket.Spec.ForProvider.Region})

	if err != nil {
		// Check to see if we already own this bucket (which happens if we run this twice)
		exists, errBucketExists := b.mc.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			return nil
		}
		// someone else might have created the bucket
		return err

	}
	return nil
}

// setLockV1Beta1 sets an annotation that tells the Observe func that we have successfully created the bucket.
func (b *bucketClient) setLockV1Beta1(bucket *miniov1beta1.Bucket) {
	if bucket.Annotations == nil {
		bucket.Annotations = map[string]string{}
	}
	bucket.Annotations[lockAnnotation] = "claimed"
}

func (b *bucketClient) emitCreationEventV1Beta1(bucket *miniov1beta1.Bucket) error {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Bucket successfully created",
	})
	return nil
}

package bucket

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (b *bucketClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("creating resource")

	bucket, ok := mg.(*miniov1beta1.Bucket)
	if !ok {
		return managed.ExternalCreation{}, errNotBucket
	}

	log.V(1).Info("Creating bucket", "name", bucket.Name)

	isAdopted, err := b.createS3Bucket(ctx, bucket)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	if bucket.Spec.ForProvider.Policy != nil {
		err = b.mc.SetBucketPolicy(ctx, bucket.GetBucketName(), *bucket.Spec.ForProvider.Policy)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	if bucket.Spec.ForProvider.Tags != nil {
		bucketTags, err := tags.NewTags(bucket.Spec.ForProvider.Tags, false)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
		err = b.mc.SetBucketTagging(ctx, bucket.GetBucketName(), bucketTags)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	b.setLock(bucket)
	if isAdopted {
		return managed.ExternalCreation{}, b.emitAdoptionEvent(bucket)
	}
	return managed.ExternalCreation{}, b.emitCreationEvent(bucket)
}

// createS3Bucket creates a new bucket and sets the name in the status.
// If the bucket already exists, and we have permissions to access it, no error is returned and the name is set in the status.
// If the bucket exists, but we don't own it, an error is returned.
// Returns (isAdopted, error) - isAdopted is true when an existing bucket was adopted rather than created new.
func (b *bucketClient) createS3Bucket(ctx context.Context, bucket *miniov1beta1.Bucket) (bool, error) {
	bucketName := bucket.GetBucketName()
	err := b.mc.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucket.Spec.ForProvider.Region})

	if err != nil {
		// Check to see if we already own this bucket (which happens if we run this twice)
		exists, errBucketExists := b.mc.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			return true, nil // adopted existing bucket
		}
		// someone else might have created the bucket
		return false, err

	}
	return false, nil // created new bucket
}

// setLock sets an annotation that tells the Observe func that we have successfully created the bucket.
// Without it, another resource that has the same bucket name might "adopt" the same bucket, causing 2 resources managing 1 bucket.
func (b *bucketClient) setLock(bucket *miniov1beta1.Bucket) {
	if bucket.Annotations == nil {
		bucket.Annotations = map[string]string{}
	}
	bucket.Annotations[lockAnnotation] = "claimed"
}

func (b *bucketClient) emitCreationEvent(bucket *miniov1beta1.Bucket) error {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Bucket successfully created",
	})
	return nil
}

func (b *bucketClient) emitAdoptionEvent(bucket *miniov1beta1.Bucket) error {
	b.recorder.Event(bucket, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Adopted",
		Message: "Existing bucket successfully adopted",
	})
	return nil
}

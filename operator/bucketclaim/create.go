package bucketclaim

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/minio/minio-go/v7"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (b *bucketClaimClient) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("creating bucket claim resource")

	bucketClaim, ok := mg.(*miniov1beta1.BucketClaim)
	if !ok {
		return managed.ExternalCreation{}, errNotBucketClaim
	}

	log.V(1).Info("Creating bucket for claim", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)

	err := b.createS3Bucket(ctx, bucketClaim)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	if bucketClaim.Spec.Policy != nil {
		err = b.mc.SetBucketPolicy(ctx, bucketClaim.GetBucketName(), *bucketClaim.Spec.Policy)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	b.setLock(bucketClaim)
	return managed.ExternalCreation{}, b.emitCreationEvent(bucketClaim)
}

// createS3Bucket creates a new bucket and sets the name in the status.
// If the bucket already exists, and we have permissions to access it, no error is returned and the name is set in the status.
// If the bucket exists, but we don't own it, an error is returned.
func (b *bucketClaimClient) createS3Bucket(ctx context.Context, bucketClaim *miniov1beta1.BucketClaim) error {
	bucketName := bucketClaim.GetBucketName()
	region := bucketClaim.Spec.Region
	if region == "" {
		region = "us-east-1"
	}

	err := b.mc.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: region})

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

// setLock sets an annotation that tells the Observe func that we have successfully created the bucket.
// Without it, another resource that has the same bucket name might "adopt" the same bucket, causing 2 resources managing 1 bucket.
func (b *bucketClaimClient) setLock(bucketClaim *miniov1beta1.BucketClaim) {
	if bucketClaim.Annotations == nil {
		bucketClaim.Annotations = map[string]string{}
	}
	bucketClaim.Annotations[bucketClaimLockAnnotation] = "claimed"
}

func (b *bucketClaimClient) emitCreationEvent(bucketClaim *miniov1beta1.BucketClaim) error {
	b.recorder.Event(bucketClaim, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Bucket successfully created for claim",
	})
	return nil
}

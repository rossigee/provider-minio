package bucket

import (
	"context"
	"net/http"
	"reflect"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// bucketExists reports whether the bucket exists in MinIO.
func (d *bucketClient) bucketExists(ctx context.Context, bucketName string) (bool, error) {
	return d.mc.BucketExists(ctx, bucketName)
}

// bucketPolicyLatest reports whether the bucket's current policy matches the
// desired policy exactly.
func (d *bucketClient) bucketPolicyLatest(ctx context.Context, bucketName, policy string) (bool, error) {
	current, err := d.mc.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return false, err
	}

	return current == policy, nil
}

// bucketTagsLatest reports whether the bucket's current tags match the desired
// tags exactly.
func (d *bucketClient) bucketTagsLatest(ctx context.Context, bucketName string, desiredTags map[string]string) (bool, error) {
	current, err := d.mc.GetBucketTagging(ctx, bucketName)
	if err != nil {
		// MinIO returns NoSuchTagSet when no tags are set
		if minio.ToErrorResponse(err).Code == "NoSuchTagSet" {
			return len(desiredTags) == 0, nil
		}
		return false, err
	}
	if current == nil {
		return len(desiredTags) == 0, nil
	}
	return reflect.DeepEqual(current.ToMap(), desiredTags), nil
}

func (d *bucketClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("observing resource")

	bucket, ok := mg.(*miniov1beta1.Bucket)
	if !ok {
		return managed.ExternalObservation{}, errNotBucket
	}

	log.V(1).Info("Observing bucket", "name", bucket.Name)
	bucketName := bucket.GetBucketName()
	exists, err := d.bucketExists(ctx, bucketName)

	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.StatusCode == http.StatusForbidden {
			// As we have full control over the minio instance, we can say with confidence that this case is a
			// "permission denied"
			return managed.ExternalObservation{}, errors.Wrap(err, "permission denied, please check the provider-config")
		}
		if errResp.StatusCode == http.StatusMovedPermanently {
			return managed.ExternalObservation{}, errors.Wrap(err, "mismatching endpointURL and zone, or bucket exists already in a different region, try changing bucket name")
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether bucket exists")
	}

	return d.observeBucket(ctx, bucket, bucketName, exists)
}

func (d *bucketClient) observeBucket(ctx context.Context, bucket *miniov1beta1.Bucket, bucketName string, exists bool) (managed.ExternalObservation, error) {
	if _, hasAnnotation := bucket.GetAnnotations()[lockAnnotation]; hasAnnotation && exists {
		bucket.Status.AtProvider.BucketName = bucketName
		bucket.SetConditions(xpv1.Available())

		isLatest := true
		if bucket.Spec.ForProvider.Policy != nil {
			u, err := d.bucketPolicyLatest(ctx, bucketName, *bucket.Spec.ForProvider.Policy)
			if err != nil {
				return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether a bucket policy exists")
			}
			isLatest = u
		}

		if isLatest && bucket.Spec.ForProvider.Tags != nil {
			u, err := d.bucketTagsLatest(ctx, bucketName, bucket.Spec.ForProvider.Tags)
			if err != nil {
				return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether bucket tags are up to date")
			}
			isLatest = u
		}

		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: isLatest}, nil
	} else if exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	return managed.ExternalObservation{}, nil
}

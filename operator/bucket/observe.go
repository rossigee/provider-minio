package bucket

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var bucketExistsFn = func(ctx context.Context, mc *minio.Client, bucketName string) (bool, error) {
	return mc.BucketExists(ctx, bucketName)
}

var bucketPolicyLatestFn = func(ctx context.Context, mc *minio.Client, bucketName string, policy string) (bool, error) {
	current, err := mc.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return false, err
	}

	return current == policy, nil
}

var bucketTagsLatestFn = func(ctx context.Context, mc *minio.Client, bucketName string, desiredTags map[string]string) (bool, error) {
	current, err := mc.GetBucketTagging(ctx, bucketName)
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
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("observing resource")

	bucket, ok := mg.(*miniov1beta1.Bucket)
	if !ok {
		return managed.ExternalObservation{}, errNotBucket
	}

	log.V(1).Info("Observing bucket", "name", bucket.Name)
	bucketName := bucket.GetBucketName()
	exists, err := bucketExistsFn(ctx, d.mc, bucketName)

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
			u, err := bucketPolicyLatestFn(ctx, d.mc, bucketName, *bucket.Spec.ForProvider.Policy)
			if err != nil {
				return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether a bucket policy exists")
			}
			isLatest = u
		}

		if isLatest && bucket.Spec.ForProvider.Tags != nil {
			u, err := bucketTagsLatestFn(ctx, d.mc, bucketName, bucket.Spec.ForProvider.Tags)
			if err != nil {
				return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether bucket tags are up to date")
			}
			isLatest = u
		}

		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: isLatest}, nil
	} else if exists {
		return managed.ExternalObservation{}, fmt.Errorf("bucket already exists, try changing bucket name: %s", bucketName)
	}

	return managed.ExternalObservation{}, nil
}

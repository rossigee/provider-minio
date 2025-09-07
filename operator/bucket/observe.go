package bucket

import (
	"context"
	"fmt"
	"net/http"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	miniov1 "github.com/rossigee/provider-minio/apis/minio/v1"
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

func (d *bucketClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("observing resource")

	var bucketName string

	// Handle both v1 and v1beta1 API versions
	if bucketv1, ok := mg.(*miniov1.Bucket); ok {
		log.V(1).Info("Observing v1 bucket", "name", bucketv1.Name)
		bucketName = bucketv1.GetBucketName()
	} else if bucketv1beta1, ok := mg.(*miniov1beta1.Bucket); ok {
		log.V(1).Info("Observing v1beta1 bucket", "name", bucketv1beta1.Name)
		bucketName = bucketv1beta1.GetBucketName()
	} else {
		return managed.ExternalObservation{}, errNotBucket
	}
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
	// Handle the rest of the logic for both API versions
	if bucketv1, ok := mg.(*miniov1.Bucket); ok {
		return d.observeBucketV1(ctx, bucketv1, bucketName, exists)
	} else if bucketv1beta1, ok := mg.(*miniov1beta1.Bucket); ok {
		return d.observeBucketV1Beta1(ctx, bucketv1beta1, bucketName, exists)
	}

	return managed.ExternalObservation{}, errNotBucket
}

func (d *bucketClient) observeBucketV1(ctx context.Context, bucket *miniov1.Bucket, bucketName string, exists bool) (managed.ExternalObservation, error) {
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

		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: isLatest}, nil
	} else if exists {
		return managed.ExternalObservation{}, fmt.Errorf("bucket already exists, try changing bucket name: %s", bucketName)
	}

	return managed.ExternalObservation{}, nil
}

func (d *bucketClient) observeBucketV1Beta1(ctx context.Context, bucket *miniov1beta1.Bucket, bucketName string, exists bool) (managed.ExternalObservation, error) {
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

		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: isLatest}, nil
	} else if exists {
		return managed.ExternalObservation{}, fmt.Errorf("bucket already exists, try changing bucket name: %s", bucketName)
	}

	return managed.ExternalObservation{}, nil
}

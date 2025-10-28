package bucketclaim

import (
	"context"
	"fmt"
	"net/http"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var bucketClaimExistsFn = func(ctx context.Context, mc *minio.Client, bucketName string) (bool, error) {
	return mc.BucketExists(ctx, bucketName)
}

var bucketClaimPolicyLatestFn = func(ctx context.Context, mc *minio.Client, bucketName string, policy string) (bool, error) {
	current, err := mc.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return false, err
	}

	return current == policy, nil
}

func (d *bucketClaimClient) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("observing bucket claim resource")

	bucketClaim, ok := mg.(*miniov1beta1.BucketClaim)
	if !ok {
		return managed.ExternalObservation{}, errNotBucketClaim
	}

	log.V(1).Info("Observing bucket claim", "name", bucketClaim.Name, "namespace", bucketClaim.Namespace)
	bucketName := bucketClaim.GetBucketName()
	exists, err := bucketClaimExistsFn(ctx, d.mc, bucketName)

	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.StatusCode == http.StatusForbidden {
			// Permission denied
			return managed.ExternalObservation{}, errors.Wrap(err, "permission denied, please check the credentials secret")
		}
		if errResp.StatusCode == http.StatusMovedPermanently {
			return managed.ExternalObservation{}, errors.Wrap(err, "mismatching endpointURL and zone, or bucket exists already in a different region, try changing bucket name")
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot determine whether bucket exists")
	}

	return d.observeBucketClaim(ctx, bucketClaim, bucketName, exists)
}

func (d *bucketClaimClient) observeBucketClaim(ctx context.Context, bucketClaim *miniov1beta1.BucketClaim, bucketName string, exists bool) (managed.ExternalObservation, error) {
	if _, hasAnnotation := bucketClaim.GetAnnotations()[bucketClaimLockAnnotation]; hasAnnotation && exists {
		bucketClaim.Status.AtProvider.BucketName = bucketName
		bucketClaim.SetConditions(xpv1.Available())

		isLatest := true
		if bucketClaim.Spec.Policy != nil {
			u, err := bucketClaimPolicyLatestFn(ctx, d.mc, bucketName, *bucketClaim.Spec.Policy)
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

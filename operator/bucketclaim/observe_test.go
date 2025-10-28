package bucketclaim

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
)

func TestBucketClaim_Observe(t *testing.T) {
	policy := "policy-struct"
	tests := map[string]struct {
		givenBucketClaim *miniov1beta1.BucketClaim
		bucketExists     bool
		returnError      error
		policyLatest     bool

		expectedError                  string
		expectedResult                 managed.ExternalObservation
		expectedBucketClaimObservation miniov1beta1.BucketClaimProviderStatus
	}{
		"NewBucketClaimDoesntYetExistOnMinio": {
			givenBucketClaim: &miniov1beta1.BucketClaim{Spec: miniov1beta1.BucketClaimSpec{
				BucketName: "my-bucket-claim"}},
			expectedResult: managed.ExternalObservation{},
		},
		"NewBucketClaimWithPolicyDoesntYetExistOnMinio": {
			givenBucketClaim: &miniov1beta1.BucketClaim{Spec: miniov1beta1.BucketClaimSpec{
				BucketName: "my-bucket-claim-with-policy",
				Policy:     &policy}},
			expectedResult: managed.ExternalObservation{},
		},
		"BucketClaimExistsAndAccessibleWithOurCredentials": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					bucketClaimLockAnnotation: "claimed",
				}},
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "my-bucket-claim"}},
			bucketExists:                   true,
			expectedResult:                 managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			expectedBucketClaimObservation: miniov1beta1.BucketClaimProviderStatus{BucketName: "my-bucket-claim"},
		},
		"NewBucketClaimObservationThrowsGenericError": {
			givenBucketClaim: &miniov1beta1.BucketClaim{Spec: miniov1beta1.BucketClaimSpec{
				BucketName: "my-bucket-claim"}},
			returnError:    errors.New("error"),
			expectedResult: managed.ExternalObservation{},
			expectedError:  "cannot determine whether bucket exists: error",
		},
		"BucketClaimAlreadyExistsOnMinio_WithoutAccess": {
			givenBucketClaim: &miniov1beta1.BucketClaim{Spec: miniov1beta1.BucketClaimSpec{
				BucketName: "my-bucket-claim"}},
			returnError:    minio.ErrorResponse{StatusCode: http.StatusForbidden, Message: "Access Denied"},
			expectedResult: managed.ExternalObservation{},
			expectedError:  "permission denied, please check the credentials secret: Access Denied",
		},
		"BucketClaimAlreadyExistsOnMinio_WithAccess_PreventAdoption": {
			// this is a case where we should avoid adopting an existing bucket even if we have access.
			// Otherwise, there could be multiple K8s resources that manage the same bucket.
			givenBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "my-bucket-claim"},
				// no bucket name in status here.
			},
			bucketExists:   true,
			expectedResult: managed.ExternalObservation{},
			expectedError:  "bucket already exists, try changing bucket name: my-bucket-claim",
		},
		"BucketClaimAlreadyExistsOnMinio_InAnotherZone": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "my-bucket-claim"}},
			returnError:    minio.ErrorResponse{StatusCode: http.StatusMovedPermanently, Message: "301 Moved Permanently"},
			expectedResult: managed.ExternalObservation{},
			expectedError:  "mismatching endpointURL and zone, or bucket exists already in a different region, try changing bucket name: 301 Moved Permanently",
		},
		"BucketClaimPolicyNoChangeRequired": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					bucketClaimLockAnnotation: "claimed",
				}},
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "my-bucket-claim",
					Policy:     &policy}},
			policyLatest:                   true,
			bucketExists:                   true,
			expectedResult:                 managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			expectedBucketClaimObservation: miniov1beta1.BucketClaimProviderStatus{BucketName: "my-bucket-claim"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			currFn := bucketClaimExistsFn
			defer func() {
				bucketClaimExistsFn = currFn
			}()

			bucketClaimPolicyLatestFn = func(ctx context.Context, mc *minio.Client, bucketName string, policy string) (bool, error) {
				return tc.policyLatest, tc.returnError
			}

			bucketClaimExistsFn = func(ctx context.Context, mc *minio.Client, bucketName string) (bool, error) {
				return tc.bucketExists, tc.returnError
			}
			b := bucketClaimClient{}
			result, err := b.Observe(logr.NewContext(context.Background(), logr.Discard()), tc.givenBucketClaim)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
			assert.Equal(t, tc.expectedBucketClaimObservation, tc.givenBucketClaim.Status.AtProvider)
		})
	}
}

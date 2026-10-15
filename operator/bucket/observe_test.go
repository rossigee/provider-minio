package bucket

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisioningPipeline_Observe(t *testing.T) {
	policy := "policy-struct"
	tests := map[string]struct {
		givenBucket  *miniov1beta1.Bucket
		bucketExists bool
		returnError  error
		policyLatest bool

		expectedError             string
		expectedResult            managed.ExternalObservation
		expectedBucketObservation miniov1beta1.BucketProviderStatus
	}{
		"NewBucketDoesntYetExistOnMinio": {
			givenBucket: &miniov1beta1.Bucket{Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
				BucketName: "my-bucket"}},
			},
			expectedResult: managed.ExternalObservation{},
		},
		"NewBucketWithPolicyDoesntYetExistOnMinio": {
			givenBucket: &miniov1beta1.Bucket{Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
				BucketName: "my-bucket-with-policy",
				Policy:     &policy}},
			},
			expectedResult: managed.ExternalObservation{},
		},
		"BucketExistsAndAccessibleWithOurCredentials": {
			givenBucket: &miniov1beta1.Bucket{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					lockAnnotation: "claimed",
				}},
				Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
					BucketName: "my-bucket"}},
			},
			bucketExists:              true,
			expectedResult:            managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			expectedBucketObservation: miniov1beta1.BucketProviderStatus{BucketName: "my-bucket"},
		},
		"NewBucketObservationThrowsGenericError": {
			givenBucket: &miniov1beta1.Bucket{Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
				BucketName: "my-bucket"}},
			},
			returnError:    errors.New("error"),
			expectedResult: managed.ExternalObservation{},
			expectedError:  "cannot determine whether bucket exists: error",
		},
		"BucketAlreadyExistsOnMinio_WithoutAccess": {
			givenBucket: &miniov1beta1.Bucket{Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
				BucketName: "my-bucket"}},
			},
			returnError:    minio.ErrorResponse{StatusCode: http.StatusForbidden, Message: "Access Denied"},
			expectedResult: managed.ExternalObservation{},
			expectedError:  "permission denied, please check the provider-config: Access Denied",
		},
		"BucketAlreadyExistsOnMinio_WithAccess_AllowAdoption": {
			givenBucket: &miniov1beta1.Bucket{
				Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
					BucketName: "my-bucket"}},
			},
			bucketExists:   true,
			expectedResult: managed.ExternalObservation{ResourceExists: false},
			expectedError:  "",
		},
		"BucketAlreadyExistsOnMinio_InAnotherZone": {
			givenBucket: &miniov1beta1.Bucket{
				Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
					BucketName: "my-bucket"}},
			},
			returnError:    minio.ErrorResponse{StatusCode: http.StatusMovedPermanently, Message: "301 Moved Permanently"},
			expectedResult: managed.ExternalObservation{},
			expectedError:  "mismatching endpointURL and zone, or bucket exists already in a different region, try changing bucket name: 301 Moved Permanently",
		},
		"BucketPolicyNoChangeRequired": {
			givenBucket: &miniov1beta1.Bucket{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					lockAnnotation: "claimed",
				}},
				Spec: miniov1beta1.BucketSpec{ForProvider: miniov1beta1.BucketParameters{
					BucketName: "my-bucket",
					Policy:     &policy}},
			},
			policyLatest:              true,
			bucketExists:              true,
			expectedResult:            managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
			expectedBucketObservation: miniov1beta1.BucketProviderStatus{BucketName: "my-bucket"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Inject a fake through the interface instead of mutating package
			// level function variables. The previous approach leaked globals
			// between tests: only bucketExistsFn was saved and restored, while
			// bucketPolicyLatestFn was overwritten and left mutated, and
			// bucketTagsLatestFn was never stubbed at all.
			mc := &fakeBucketS3{
				exists: tc.bucketExists,
				err:    tc.returnError,
			}
			// When the case expects the policy to be up to date, report back
			// exactly the desired policy so the equality check passes.
			if tc.policyLatest && tc.givenBucket.Spec.ForProvider.Policy != nil {
				mc.currentPolicy = *tc.givenBucket.Spec.ForProvider.Policy
			}
			b := bucketClient{mc: mc}
			result, err := b.Observe(logr.NewContext(context.Background(), logr.Discard()), tc.givenBucket)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
			assert.Equal(t, tc.expectedBucketObservation, tc.givenBucket.Status.AtProvider)
		})
	}
}

// fakeBucketS3 is a scriptable minioutil.BucketS3 covering the methods Observe
// exercises. BucketExists drives the existence check; GetBucketPolicy and
// GetBucketTagging are backed by the scriptable policyLatest/exists fields.
type fakeBucketS3 struct {
	exists        bool
	currentPolicy string
	err           error
}

func (f *fakeBucketS3) BucketExists(_ context.Context, _ string) (bool, error) {
	return f.exists, f.err
}

func (f *fakeBucketS3) GetBucketPolicy(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.currentPolicy, nil
}

func (f *fakeBucketS3) GetBucketTagging(_ context.Context, _ string) (*tags.Tags, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeBucketS3) MakeBucket(context.Context, string, minio.MakeBucketOptions) error {
	return f.err
}
func (f *fakeBucketS3) SetBucketPolicy(context.Context, string, string) error      { return f.err }
func (f *fakeBucketS3) SetBucketTagging(context.Context, string, *tags.Tags) error { return f.err }
func (f *fakeBucketS3) RemoveBucketTagging(context.Context, string) error          { return f.err }
func (f *fakeBucketS3) RemoveBucket(context.Context, string) error                 { return f.err }

func (f *fakeBucketS3) ListObjects(context.Context, string, minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	out := make(chan minio.ObjectInfo)
	close(out)
	return out
}

func (f *fakeBucketS3) RemoveObjects(_ context.Context, _ string, objectsCh <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	out := make(chan minio.RemoveObjectError)
	go func() {
		defer close(out)
		for range objectsCh { //nolint:revive // draining is the point
		}
	}()
	return out
}

func (f *fakeBucketS3) GetObjectLockConfig(context.Context, string) (string, *minio.RetentionMode, *uint, *minio.ValidityUnit, error) {
	return "", nil, nil, nil, f.err
}

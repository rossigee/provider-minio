package bucketclaim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
)

func TestBucketClaim_GetBucketName(t *testing.T) {
	tests := map[string]struct {
		givenBucketClaim *miniov1beta1.BucketClaim
		expectedName     string
	}{
		"BucketNameSpecified": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bucket-claim",
				},
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "custom-bucket-name",
				},
			},
			expectedName: "custom-bucket-name",
		},
		"BucketNameNotSpecified": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bucket-claim",
				},
				Spec: miniov1beta1.BucketClaimSpec{},
			},
			expectedName: "test-bucket-claim",
		},
		"EmptyBucketName": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bucket-claim",
				},
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "",
				},
			},
			expectedName: "test-bucket-claim",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.givenBucketClaim.GetBucketName()
			assert.Equal(t, tc.expectedName, result)
		})
	}
}

func TestBucketClaim_SetLock(t *testing.T) {
	tests := map[string]struct {
		givenBucketClaim    *miniov1beta1.BucketClaim
		expectedAnnotations map[string]string
	}{
		"NoExistingAnnotations": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bucket-claim",
				},
			},
			expectedAnnotations: map[string]string{
				bucketClaimLockAnnotation: "claimed",
			},
		},
		"ExistingAnnotations": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bucket-claim",
					Annotations: map[string]string{
						"existing-annotation": "value",
					},
				},
			},
			expectedAnnotations: map[string]string{
				"existing-annotation":     "value",
				bucketClaimLockAnnotation: "claimed",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := &bucketClaimClient{}
			b.setLock(tc.givenBucketClaim)

			for key, expectedValue := range tc.expectedAnnotations {
				actualValue, exists := tc.givenBucketClaim.Annotations[key]
				assert.True(t, exists, "Annotation %s should exist", key)
				assert.Equal(t, expectedValue, actualValue, "Annotation %s should have value %s", key, expectedValue)
			}
		})
	}
}

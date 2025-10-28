package bucketclaim

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/stretchr/testify/assert"

	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
)

func TestBucketClaim_Validator(t *testing.T) {
	tests := map[string]struct {
		givenBucketClaim *miniov1beta1.BucketClaim
		expectedError    string
	}{
		"ValidBucketClaim": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
				},
			},
		},
		"MissingCredentialsSecretRef": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "test-bucket",
				},
			},
			expectedError: ".spec.credentialsSecretRef.name is required",
		},
		"EmptyCredentialsSecretName": {
			givenBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "",
					},
					BucketName: "test-bucket",
				},
			},
			expectedError: ".spec.credentialsSecretRef.name is required",
		},
	}

	v := &Validator{}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := v.ValidateCreate(context.Background(), tc.givenBucketClaim)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBucketClaim_ValidateUpdate(t *testing.T) {
	tests := map[string]struct {
		oldBucketClaim *miniov1beta1.BucketClaim
		newBucketClaim *miniov1beta1.BucketClaim
		expectedError  string
	}{
		"ValidUpdate": {
			oldBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
					Region:     "us-east-1",
				},
			},
			newBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
					Region:     "us-west-2",
				},
			},
		},
		"BucketNameChanged": {
			oldBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
				},
				Status: miniov1beta1.BucketClaimStatus{
					AtProvider: miniov1beta1.BucketClaimProviderStatus{
						BucketName: "test-bucket",
					},
				},
			},
			newBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "changed-bucket",
				},
			},
			expectedError: "Changing the bucket name is not allowed after creation",
		},
		"RegionChanged": {
			oldBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
					Region:     "us-east-1",
				},
				Status: miniov1beta1.BucketClaimStatus{
					AtProvider: miniov1beta1.BucketClaimProviderStatus{
						BucketName: "test-bucket",
					},
				},
			},
			newBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
					Region:     "us-west-2",
				},
			},
			expectedError: "Changing the region is not allowed after creation",
		},
		"MissingCredentialsSecretRef": {
			oldBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
				},
				Status: miniov1beta1.BucketClaimStatus{
					AtProvider: miniov1beta1.BucketClaimProviderStatus{
						BucketName: "test-bucket",
					},
				},
			},
			newBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					BucketName: "test-bucket",
				},
			},
			expectedError: "Credentials secret reference is required",
		},
		"AllowUpdateBeforeCreation": {
			oldBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "test-bucket",
				},
			},
			newBucketClaim: &miniov1beta1.BucketClaim{
				Spec: miniov1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name: "test-secret",
					},
					BucketName: "changed-bucket",
				},
			},
		},
	}

	v := &Validator{}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := v.ValidateUpdate(context.Background(), tc.oldBucketClaim, tc.newBucketClaim)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBucketClaim_ValidateDelete(t *testing.T) {
	v := &Validator{}
	bucketClaim := &miniov1beta1.BucketClaim{}

	_, err := v.ValidateDelete(context.Background(), bucketClaim)
	assert.NoError(t, err)
}

func TestBucketClaim_ValidateCreate_WrongType(t *testing.T) {
	v := &Validator{}
	wrongType := &miniov1beta1.Bucket{}

	_, err := v.ValidateCreate(context.Background(), wrongType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "managed resource is not a bucket claim")
}

func TestBucketClaim_ValidateUpdate_WrongType(t *testing.T) {
	v := &Validator{}
	wrongType := &miniov1beta1.Bucket{}

	_, err := v.ValidateUpdate(context.Background(), wrongType, wrongType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "managed resource is not a bucket claim")
}

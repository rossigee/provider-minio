package serviceaccount

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name          string
		serviceAccount *miniov1.ServiceAccount
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid ServiceAccount - should pass",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						Name:        "Test Service Account",
						Description: "Test Description",
						AccessKey:   "myaccessid",
						SecretKey:   "myaccessdata",
						Policy:      `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::bucket/*"]}]}`,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Missing ProviderConfigReference - should fail",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						Name: "Test Service Account",
					},
				},
			},
			expectedError: true,
			errorContains: "Provider config is required",
		},
		{
			name: "Invalid access key - too short",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						AccessKey: "ab", // Too short
					},
				},
			},
			expectedError: true,
			errorContains: "Access key must be between 3 and 128 characters",
		},
		{
			name: "Invalid secret key - too short",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						SecretKey: "short", // Too short (less than 8 characters)
					},
				},
			},
			expectedError: true,
			errorContains: "Secret key must be at least 8 characters",
		},
		{
			name: "Invalid JSON policy - should fail",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						Policy: `invalid json {`,
					},
				},
			},
			expectedError: true,
			errorContains: "policy must be valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &Validator{
				log:  testr.New(t),
				kube: fake.NewClientBuilder().Build(),
			}

			_, err := validator.ValidateCreate(context.Background(), tt.serviceAccount)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name              string
		oldServiceAccount *miniov1.ServiceAccount
		newServiceAccount *miniov1.ServiceAccount
		expectedError     bool
		errorContains     string
	}{
		{
			name: "Valid update - should pass",
			oldServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						AccessKey:  "myaccessid",
						TargetUser: "test-user",
					},
				},
			},
			newServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						AccessKey:   "myaccessid", // Same access key
						TargetUser:  "test-user",       // Same target user
						Description: "Updated description", // This can change
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Changing access key - should fail",
			oldServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						AccessKey: "olduser",
					},
				},
			},
			newServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						AccessKey: "newuser", // Changed access key
					},
				},
			},
			expectedError: true,
			errorContains: "Changing the access key is not allowed",
		},
		{
			name: "Changing target user - should fail",
			oldServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						TargetUser: "old-user",
					},
				},
			},
			newServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
					ForProvider: miniov1.ServiceAccountParameters{
						TargetUser: "new-user", // Changed target user
					},
				},
			},
			expectedError: true,
			errorContains: "Changing the target user is not allowed",
		},
		{
			name: "Update during deletion - should pass",
			oldServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-serviceaccount",
				},
			},
			newServiceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-serviceaccount",
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
				},
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-provider-config",
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &Validator{
				log:  testr.New(t),
				kube: fake.NewClientBuilder().Build(),
			}

			_, err := validator.ValidateUpdate(context.Background(), tt.oldServiceAccount, tt.newServiceAccount)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateDelete(t *testing.T) {
	validator := &Validator{
		log:  testr.New(t),
		kube: fake.NewClientBuilder().Build(),
	}

	serviceAccount := &miniov1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-serviceaccount",
		},
	}

	_, err := validator.ValidateDelete(context.Background(), serviceAccount)
	assert.NoError(t, err)
}

func TestValidator_ValidatePolicy(t *testing.T) {
	validator := &Validator{
		log:  testr.New(t),
		kube: fake.NewClientBuilder().Build(),
	}

	serviceAccount := &miniov1.ServiceAccount{}

	tests := []struct {
		name          string
		policy        string
		expectedError bool
	}{
		{
			name:          "Valid JSON policy",
			policy:        `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::bucket/*"]}]}`,
			expectedError: false,
		},
		{
			name:          "Invalid JSON policy",
			policy:        `invalid json {`,
			expectedError: true,
		},
		{
			name:          "Empty policy",
			policy:        "",
			expectedError: false,
		},
		{
			name:          "Valid but empty JSON policy",
			policy:        `{}`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validatePolicy(context.Background(), serviceAccount, tt.policy)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

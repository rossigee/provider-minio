package serviceaccount

import (
	"context"
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/minio/madmin-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestObserve(t *testing.T) {
	testCases := []struct {
		name           string
		serviceAccount *miniov1.ServiceAccount
		setupMocks     func(*MockMinioAdminClient)
		expectedError  string
		validateResult func(*testing.T, managed.ExternalObservation, *miniov1.ServiceAccount)
	}{
		{
			name: "ResourceDoesNotExist_NoExternalName",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				// Should not call InfoServiceAccount when no external name
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.False(t, result.ResourceExists)
				assert.False(t, result.ResourceUpToDate)
				assert.Empty(t, result.ConnectionDetails)
			},
		},
		{
			name: "ResourceDoesNotExist_NotFoundInMinIO",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{}, errors.New("The specified service account does not exist"))
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.False(t, result.ResourceExists)
				assert.Empty(t, result.ConnectionDetails)
			},
		},
		{
			name: "ResourceExists_UpToDate",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						Policies:   []string{"readonly", "writeonly"},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{
						ParentUser:    "testuser",
						Policy:        "readonly,writeonly",
						AccountStatus: "enabled",
					}, nil)
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.True(t, result.ResourceExists)
				assert.True(t, result.ResourceUpToDate)
				assert.Equal(t, "test-access-key", string(result.ConnectionDetails["accessKey"]))
				assert.Equal(t, "testuser", string(result.ConnectionDetails["parentUser"]))
				assert.Equal(t, "test-access-key", sa.Status.AtProvider.AccessKey)
				assert.Equal(t, "testuser", sa.Status.AtProvider.ParentUser)
				assert.Equal(t, "enabled", sa.Status.AtProvider.Status)
				assert.Equal(t, "readonly,writeonly", sa.Status.AtProvider.Policies)
				assert.Equal(t, xpv1.TypeReady, sa.Status.GetCondition(xpv1.TypeReady).Type)
				assert.Equal(t, xpv1.ReasonAvailable, sa.Status.GetCondition(xpv1.TypeReady).Reason)
			},
		},
		{
			name: "ResourceExists_NotUpToDate_DifferentParentUser",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "newuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{
						ParentUser:    "olduser",
						AccountStatus: "enabled",
					}, nil)
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.True(t, result.ResourceExists)
				assert.False(t, result.ResourceUpToDate)
				assert.Equal(t, "olduser", sa.Status.AtProvider.ParentUser)
			},
		},
		{
			name: "ResourceExists_NotUpToDate_DifferentPolicies",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						Policies:   []string{"readonly", "writeonly", "admin"},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{
						ParentUser:    "testuser",
						Policy:        "readonly,writeonly",
						AccountStatus: "enabled",
					}, nil)
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.True(t, result.ResourceExists)
				assert.False(t, result.ResourceUpToDate)
			},
		},
		{
			name: "Error_MinIOAPIFailure",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{}, errors.New("network error"))
			},
			expectedError: "failed to get service account info: network error",
		},
		{
			name: "ResourceExists_NoPolicies",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						// No policies specified
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient) {
				ma.On("InfoServiceAccount", mock.Anything, "test-access-key").Return(
					madmin.InfoServiceAccountResp{
						ParentUser:    "testuser",
						Policy:        "someotherpolicy",
						AccountStatus: "enabled",
					}, nil)
			},
			validateResult: func(t *testing.T, result managed.ExternalObservation, sa *miniov1.ServiceAccount) {
				assert.True(t, result.ResourceExists)
				assert.True(t, result.ResourceUpToDate) // Should be up to date when no policies specified
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockMA := new(MockMinioAdminClient)

			if tc.setupMocks != nil {
				tc.setupMocks(mockMA)
			}

			// Create client
			client := &serviceAccountClient{
				ma: mockMA,
			}

			// Execute
			result, err := client.Observe(context.Background(), tc.serviceAccount)

			// Validate
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				if tc.validateResult != nil {
					tc.validateResult(t, result, tc.serviceAccount)
				}
			}

			// Verify all mock expectations were met
			mockMA.AssertExpectations(t)
		})
	}
}

func TestObserve_NotServiceAccount(t *testing.T) {
	client := &serviceAccountClient{}

	// Use a different resource type
	nonSA := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-service-account",
		},
	}

	result, err := client.Observe(context.Background(), nonSA)

	assert.Error(t, err)
	assert.Equal(t, errNotServiceAccount, err)
	assert.Empty(t, result.ConnectionDetails)
}

func TestIsServiceAccountNotFound_Observe(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Contains 'does not exist'",
			err:      errors.New("The service account does not exist"),
			expected: true,
		},
		{
			name:     "Contains 'not found'",
			err:      errors.New("Service account not found"),
			expected: true,
		},
		{
			name:     "Contains 'NoSuchServiceAccount'",
			err:      errors.New("NoSuchServiceAccount: test-key"),
			expected: true,
		},
		{
			name:     "Other error",
			err:      errors.New("network timeout"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isServiceAccountNotFound(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	client := &serviceAccountClient{}

	testCases := []struct {
		name           string
		serviceAccount *miniov1.ServiceAccount
		info           madmin.InfoServiceAccountResp
		expected       bool
	}{
		{
			name: "UpToDate_SameParentAndPolicies",
			serviceAccount: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						Policies:   []string{"readonly", "writeonly"},
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				ParentUser: "testuser",
				Policy:     "readonly,writeonly",
			},
			expected: true,
		},
		{
			name: "NotUpToDate_DifferentParent",
			serviceAccount: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "newuser",
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				ParentUser: "olduser",
			},
			expected: false,
		},
		{
			name: "NotUpToDate_DifferentPolicies",
			serviceAccount: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						Policies:   []string{"readonly", "admin"},
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				ParentUser: "testuser",
				Policy:     "readonly,writeonly",
			},
			expected: false,
		},
		{
			name: "UpToDate_NoPoliciesSpecified",
			serviceAccount: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						// No policies
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				ParentUser: "testuser",
				Policy:     "somepolicy",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.isUpToDate(tc.serviceAccount, tc.info)
			assert.Equal(t, tc.expected, result)
		})
	}
}

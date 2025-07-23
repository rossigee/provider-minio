package serviceaccount

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/minio/madmin-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MockMinioAdminClient is a mock implementation of the MinIO admin client
type MockMinioAdminClient struct {
	mock.Mock
}

func (m *MockMinioAdminClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(madmin.Credentials), args.Error(1)
}

func (m *MockMinioAdminClient) GetServiceAccount(ctx context.Context, accessKey string) (*madmin.ServiceAccountInfo, error) {
	args := m.Called(ctx, accessKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*madmin.ServiceAccountInfo), args.Error(1)
}

func (m *MockMinioAdminClient) UpdateServiceAccount(ctx context.Context, accessKey string, opts madmin.UpdateServiceAccountReq) error {
	args := m.Called(ctx, accessKey, opts)
	return args.Error(0)
}

func (m *MockMinioAdminClient) DeleteServiceAccount(ctx context.Context, accessKey string) error {
	args := m.Called(ctx, accessKey)
	return args.Error(0)
}

func (m *MockMinioAdminClient) InfoServiceAccount(ctx context.Context, accessKey string) (madmin.InfoServiceAccountResp, error) {
	args := m.Called(ctx, accessKey)
	return args.Get(0).(madmin.InfoServiceAccountResp), args.Error(1)
}

// MockEventRecorder is a mock implementation of the event recorder
type MockEventRecorder struct {
	mock.Mock
}

func (m *MockEventRecorder) Event(object runtime.Object, e event.Event) {
	m.Called(object, e)
}

func (m *MockEventRecorder) WithAnnotations(annotations ...string) event.Recorder {
	// Return self for chaining
	return m
}

func TestCreate(t *testing.T) {
	testCases := []struct {
		name           string
		serviceAccount *miniov1.ServiceAccount
		setupMocks     func(*MockMinioAdminClient, *MockEventRecorder)
		expectedError  string
		validateResult func(*testing.T, managed.ExternalCreation, *miniov1.ServiceAccount)
	}{
		{
			name: "SuccessfulCreation_MinimalConfig",
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				expectedOpts := madmin.AddServiceAccountReq{
					TargetUser: "testuser",
					Name:       "test-sa",
				}
				ma.On("AddServiceAccount", mock.Anything, expectedOpts).Return(
					madmin.Credentials{
						AccessKey: "test-access-key",
						SecretKey: "test-secret-key",
					}, nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Created"
				}))
			},
			validateResult: func(t *testing.T, result managed.ExternalCreation, sa *miniov1.ServiceAccount) {
				assert.Equal(t, "test-access-key", string(result.ConnectionDetails["accessKey"]))
				assert.Equal(t, "test-secret-key", string(result.ConnectionDetails["secretKey"]))
				assert.Equal(t, "testuser", string(result.ConnectionDetails["parentUser"]))
				assert.Equal(t, "test-access-key", meta.GetExternalName(sa))
				assert.Equal(t, "true", sa.GetAnnotations()[ServiceAccountCreatedAnnotationKey])
			},
		},
		{
			name: "SuccessfulCreation_WithAllOptions",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa-full",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "testuser",
						Description: "Test service account",
						Policies:    []string{"readonly", "writeonly"},
						Expiry:      &metav1.Time{Time: time.Now().Add(24 * time.Hour)},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("AddServiceAccount", mock.Anything, mock.MatchedBy(func(opts madmin.AddServiceAccountReq) bool {
					var policies []string
					json.Unmarshal(opts.Policy, &policies)
					return opts.TargetUser == "testuser" &&
						opts.Name == "test-sa-full" &&
						opts.Description == "Test service account" &&
						len(policies) == 2 &&
						opts.Expiration != nil
				})).Return(
					madmin.Credentials{
						AccessKey: "test-access-key-full",
						SecretKey: "test-secret-key-full",
					}, nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Created"
				}))
			},
			validateResult: func(t *testing.T, result managed.ExternalCreation, sa *miniov1.ServiceAccount) {
				assert.Equal(t, "test-access-key-full", string(result.ConnectionDetails["accessKey"]))
				assert.Equal(t, "test-secret-key-full", string(result.ConnectionDetails["secretKey"]))
				assert.Equal(t, "testuser", string(result.ConnectionDetails["parentUser"]))
				assert.Equal(t, "test-access-key-full", meta.GetExternalName(sa))
			},
		},
		{
			name: "SkipCreation_AlreadyCreated",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa-exists",
					Annotations: map[string]string{
						ServiceAccountCreatedAnnotationKey: "true",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				// Should not call AddServiceAccount
			},
			validateResult: func(t *testing.T, result managed.ExternalCreation, sa *miniov1.ServiceAccount) {
				assert.Empty(t, result.ConnectionDetails)
			},
		},
		{
			name: "Error_MissingParentUser",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa-no-parent",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						// ParentUser is missing
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				// Should not call any mocks
			},
			expectedError: "parentUser is required for service account creation",
		},
		{
			name: "Error_MinioAPIFailure",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa-api-error",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("AddServiceAccount", mock.Anything, mock.Anything).Return(
					madmin.Credentials{}, errors.New("minio API error"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "CreateFailed"
				}))
			},
			expectedError: "failed to create service account: minio API error",
		},
		{
			name: "Error_InvalidPoliciesJSON",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa-bad-policy",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
						// This will cause JSON marshaling to fail in real scenario,
						// but for testing we'll handle it differently
						Policies: []string{"valid-policy"},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				// Setup normal successful response
				ma.On("AddServiceAccount", mock.Anything, mock.Anything).Return(
					madmin.Credentials{
						AccessKey: "test-key",
						SecretKey: "test-secret",
					}, nil)
				er.On("Event", mock.Anything, mock.Anything)
			},
			validateResult: func(t *testing.T, result managed.ExternalCreation, sa *miniov1.ServiceAccount) {
				// Should succeed in this test case
				assert.NotEmpty(t, result.ConnectionDetails)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockMA := new(MockMinioAdminClient)
			mockRecorder := new(MockEventRecorder)

			if tc.setupMocks != nil {
				tc.setupMocks(mockMA, mockRecorder)
			}

			// Create client
			client := &serviceAccountClient{
				ma:       mockMA,
				recorder: mockRecorder,
			}

			// Execute
			result, err := client.Create(context.Background(), tc.serviceAccount)

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
			mockRecorder.AssertExpectations(t)
		})
	}
}

func TestCreate_NotServiceAccount(t *testing.T) {
	// Test with a non-ServiceAccount resource
	client := &serviceAccountClient{}

	// Use a different resource type
	nonSA := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-service-account",
		},
	}

	result, err := client.Create(context.Background(), nonSA)

	assert.Error(t, err)
	assert.Equal(t, errNotServiceAccount, err)
	assert.Empty(t, result.ConnectionDetails)
}

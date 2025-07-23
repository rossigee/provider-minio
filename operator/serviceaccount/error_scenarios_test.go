package serviceaccount

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/minio/madmin-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNetworkErrorScenarios tests various network-related error conditions
func TestNetworkErrorScenarios(t *testing.T) {
	testCases := []struct {
		name          string
		operation     string
		setupMocks    func(*MockMinioAdminClient, *MockEventRecorder)
		executeOp     func(*serviceAccountClient, *miniov1.ServiceAccount) error
		expectedError string
	}{
		{
			name:      "Create_NetworkTimeout",
			operation: "create",
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("AddServiceAccount", mock.Anything, mock.Anything).Return(
					madmin.Credentials{}, &net.OpError{Op: "dial", Err: errors.New("timeout")})
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "CreateFailed"
				}))
			},
			executeOp: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Create(context.Background(), sa)
				return err
			},
			expectedError: "failed to create service account",
		},
		{
			name:      "Observe_ConnectionRefused",
			operation: "observe",
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("InfoServiceAccount", mock.Anything, "test-key").Return(
					madmin.InfoServiceAccountResp{}, &net.OpError{Op: "dial", Err: errors.New("connection refused")})
			},
			executeOp: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Observe(context.Background(), sa)
				return err
			},
			expectedError: "failed to get service account info",
		},
		{
			name:      "Update_DNSResolutionFailure",
			operation: "update",
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-key", mock.Anything).Return(
					&net.DNSError{Err: "no such host", Name: "minio.example.com", IsNotFound: true})
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "UpdateFailed"
				}))
			},
			executeOp: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Update(context.Background(), sa)
				return err
			},
			expectedError: "failed to update service account",
		},
		{
			name:      "Delete_ContextCancelled",
			operation: "delete",
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-key").Return(
					context.Canceled)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "DeleteFailed"
				}))
			},
			executeOp: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Delete(context.Background(), sa)
				return err
			},
			expectedError: "context canceled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockMA := new(MockMinioAdminClient)
			mockRecorder := new(MockEventRecorder)

			tc.setupMocks(mockMA, mockRecorder)

			client := &serviceAccountClient{
				ma:       mockMA,
				recorder: mockRecorder,
			}

			sa := &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			}

			// Execute
			err := tc.executeOp(client, sa)

			// Validate
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)

			mockMA.AssertExpectations(t)
			mockRecorder.AssertExpectations(t)
		})
	}
}

// TestAPIErrorResponses tests various MinIO API error responses
func TestAPIErrorResponses(t *testing.T) {
	testCases := []struct {
		name          string
		apiError      error
		operation     func(*serviceAccountClient, *miniov1.ServiceAccount) error
		shouldSucceed bool
		setupMocks    func(*MockMinioAdminClient, *MockEventRecorder, error)
	}{
		{
			name:     "Create_ServiceAccountAlreadyExists",
			apiError: errors.New("ServiceAccountAlreadyExists: The service account already exists"),
			operation: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Create(context.Background(), sa)
				return err
			},
			shouldSucceed: false,
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder, apiErr error) {
				ma.On("AddServiceAccount", mock.Anything, mock.Anything).Return(
					madmin.Credentials{}, apiErr)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning
				}))
			},
		},
		{
			name:     "Observe_InvalidAccessKey",
			apiError: errors.New("InvalidAccessKeyId: The access key ID you provided does not exist"),
			operation: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Observe(context.Background(), sa)
				return err
			},
			shouldSucceed: true, // Should return ResourceExists: false
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder, apiErr error) {
				ma.On("InfoServiceAccount", mock.Anything, mock.Anything).Return(
					madmin.InfoServiceAccountResp{}, apiErr)
			},
		},
		{
			name:     "Update_InsufficientPermissions",
			apiError: errors.New("AccessDenied: User does not have permission to update service account"),
			operation: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Update(context.Background(), sa)
				return err
			},
			shouldSucceed: false,
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder, apiErr error) {
				ma.On("UpdateServiceAccount", mock.Anything, mock.Anything, mock.Anything).Return(apiErr)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning
				}))
			},
		},
		{
			name:     "Delete_AlreadyDeleted",
			apiError: errors.New("NoSuchServiceAccount: The specified service account does not exist"),
			operation: func(c *serviceAccountClient, sa *miniov1.ServiceAccount) error {
				_, err := c.Delete(context.Background(), sa)
				return err
			},
			shouldSucceed: true, // Should succeed as it's idempotent
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder, apiErr error) {
				ma.On("DeleteServiceAccount", mock.Anything, mock.Anything).Return(apiErr)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Deleted"
				}))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockMA := new(MockMinioAdminClient)
			mockRecorder := new(MockEventRecorder)

			tc.setupMocks(mockMA, mockRecorder, tc.apiError)

			client := &serviceAccountClient{
				ma:       mockMA,
				recorder: mockRecorder,
			}

			sa := &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			}

			// Execute
			err := tc.operation(client, sa)

			// Validate
			if tc.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			mockMA.AssertExpectations(t)
			mockRecorder.AssertExpectations(t)
		})
	}
}

// TestConcurrentOperations tests race conditions and concurrent operations
func TestConcurrentOperations(t *testing.T) {
	// Test concurrent updates
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		mockMA := new(MockMinioAdminClient)
		mockRecorder := new(MockEventRecorder)

		// Simulate concurrent updates - first succeeds, subsequent may fail
		mockMA.On("UpdateServiceAccount", mock.Anything, "test-key", mock.Anything).Return(nil).Once()
		mockMA.On("UpdateServiceAccount", mock.Anything, "test-key", mock.Anything).Return(
			errors.New("ConflictError: Another update is in progress")).Maybe()

		mockRecorder.On("Event", mock.Anything, mock.Anything).Maybe()

		client := &serviceAccountClient{
			ma:       mockMA,
			recorder: mockRecorder,
		}

		sa := &miniov1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-sa",
				Annotations: map[string]string{
					meta.AnnotationKeyExternalName: "test-key",
				},
			},
			Spec: miniov1.ServiceAccountSpec{
				ForProvider: miniov1.ServiceAccountParameters{
					ParentUser:  "testuser",
					Description: "Updated description",
				},
			},
		}

		// Execute concurrent updates
		errChan := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() {
				_, err := client.Update(context.Background(), sa)
				errChan <- err
			}()
		}

		// Collect results
		var errors []error
		for i := 0; i < 2; i++ {
			err := <-errChan
			if err != nil {
				errors = append(errors, err)
			}
		}

		// One should succeed, one should fail
		assert.Len(t, errors, 1)
		assert.Contains(t, errors[0].Error(), "ConflictError")
	})
}

// TestRetryableErrors tests errors that might be retryable
func TestRetryableErrors(t *testing.T) {
	retryableErrors := []struct {
		name  string
		error error
	}{
		{
			name:  "TemporarilyUnavailable",
			error: errors.New("ServiceUnavailable: The service is temporarily unavailable"),
		},
		{
			name:  "TooManyRequests",
			error: errors.New("TooManyRequests: Rate limit exceeded"),
		},
		{
			name:  "RequestTimeout",
			error: errors.New("RequestTimeout: The request timed out"),
		},
	}

	for _, tc := range retryableErrors {
		t.Run(tc.name, func(t *testing.T) {
			mockMA := new(MockMinioAdminClient)
			mockRecorder := new(MockEventRecorder)

			// Test with Create operation
			mockMA.On("AddServiceAccount", mock.Anything, mock.Anything).Return(
				madmin.Credentials{}, tc.error)
			mockRecorder.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
				return e.Type == event.TypeWarning
			}))

			client := &serviceAccountClient{
				ma:       mockMA,
				recorder: mockRecorder,
			}

			sa := &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			}

			_, err := client.Create(context.Background(), sa)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create service account")

			mockMA.AssertExpectations(t)
			mockRecorder.AssertExpectations(t)
		})
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("CreateWithExpiredTime", func(t *testing.T) {
		mockMA := new(MockMinioAdminClient)
		mockRecorder := new(MockEventRecorder)

		client := &serviceAccountClient{
			ma:       mockMA,
			recorder: mockRecorder,
		}

		expiredTime := time.Now().Add(-24 * time.Hour) // Past time
		sa := &miniov1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-sa",
			},
			Spec: miniov1.ServiceAccountSpec{
				ForProvider: miniov1.ServiceAccountParameters{
					ParentUser: "testuser",
					Expiry:     &metav1.Time{Time: expiredTime},
				},
			},
		}

		mockMA.On("AddServiceAccount", mock.Anything, mock.MatchedBy(func(opts madmin.AddServiceAccountReq) bool {
			return opts.Expiration != nil && opts.Expiration.Before(time.Now())
		})).Return(madmin.Credentials{}, errors.New("InvalidExpiryDate: Expiry date cannot be in the past"))
		mockRecorder.On("Event", mock.Anything, mock.Anything)

		_, err := client.Create(context.Background(), sa)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create service account")
	})

	t.Run("ObserveWithCorruptedResponse", func(t *testing.T) {
		mockMA := new(MockMinioAdminClient)
		client := &serviceAccountClient{
			ma: mockMA,
		}

		sa := &miniov1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-sa",
				Annotations: map[string]string{
					meta.AnnotationKeyExternalName: "test-key",
				},
			},
			Spec: miniov1.ServiceAccountSpec{
				ForProvider: miniov1.ServiceAccountParameters{
					ParentUser: "testuser",
				},
			},
		}

		// Return a response with empty/invalid data
		mockMA.On("InfoServiceAccount", mock.Anything, "test-key").Return(
			madmin.InfoServiceAccountResp{
				// All fields empty - corrupted response
			}, nil)

		result, err := client.Observe(context.Background(), sa)
		assert.NoError(t, err)
		assert.True(t, result.ResourceExists)
		assert.False(t, result.ResourceUpToDate) // Parent user won't match
	})
}

package serviceaccount

import (
	"context"
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDelete(t *testing.T) {
	testCases := []struct {
		name           string
		serviceAccount *miniov1.ServiceAccount
		setupMocks     func(*MockMinioAdminClient, *MockEventRecorder)
		expectedError  string
		validateResult func(*testing.T, *miniov1.ServiceAccount)
	}{
		{
			name: "SuccessfulDeletion",
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-access-key").Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Deleted"
				}))
			},
			validateResult: func(t *testing.T, sa *miniov1.ServiceAccount) {
				// Check that deleting condition was set
				assert.Equal(t, xpv1.TypeReady, sa.Status.GetCondition(xpv1.TypeReady).Type)
				assert.Equal(t, xpv1.ReasonDeleting, sa.Status.GetCondition(xpv1.TypeReady).Reason)
			},
		},
		{
			name: "AlreadyDeleted_ServiceAccountNotFound",
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-access-key").Return(
					errors.New("The specified service account does not exist"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Deleted"
				}))
			},
			validateResult: func(t *testing.T, sa *miniov1.ServiceAccount) {
				// Should succeed even if already deleted
			},
		},
		{
			name: "NoExternalName_NothingToDelete",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					// No external name annotation
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "testuser",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				// Should not call DeleteServiceAccount
			},
			validateResult: func(t *testing.T, sa *miniov1.ServiceAccount) {
				// Should succeed without doing anything
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-access-key").Return(
					errors.New("minio API error"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "DeleteFailed"
				}))
			},
			expectedError: "minio API error",
		},
		{
			name: "AlreadyDeleted_NotFoundVariant",
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-access-key").Return(
					errors.New("NoSuchServiceAccount: test-access-key"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Deleted"
				}))
			},
			validateResult: func(t *testing.T, sa *miniov1.ServiceAccount) {
				// Should succeed even if already deleted
			},
		},
		{
			name: "Error_NetworkTimeout",
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
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("DeleteServiceAccount", mock.Anything, "test-access-key").Return(
					errors.New("network timeout"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "DeleteFailed"
				}))
			},
			expectedError: "network timeout",
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
			result, err := client.Delete(context.Background(), tc.serviceAccount)

			// Validate
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, result) // Delete returns empty ExternalDelete
			}

			if tc.validateResult != nil {
				tc.validateResult(t, tc.serviceAccount)
			}

			// Verify all mock expectations were met
			mockMA.AssertExpectations(t)
			mockRecorder.AssertExpectations(t)
		})
	}
}

func TestDelete_NotServiceAccount(t *testing.T) {
	client := &serviceAccountClient{}

	// Use a different resource type
	nonSA := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-service-account",
		},
	}

	result, err := client.Delete(context.Background(), nonSA)

	assert.Error(t, err)
	assert.Equal(t, errNotServiceAccount, err)
	assert.Empty(t, result)
}

func TestEventEmission(t *testing.T) {
	// Test that event emission functions work correctly
	mockRecorder := new(MockEventRecorder)
	client := &serviceAccountClient{
		recorder: mockRecorder,
	}

	sa := &miniov1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa",
		},
	}

	// Test deletion event
	mockRecorder.On("Event", sa, mock.MatchedBy(func(e event.Event) bool {
		return e.Type == event.TypeNormal &&
			e.Reason == "Deleted" &&
			e.Message == "Service account successfully deleted"
	}))
	client.emitDeletionEvent(sa)

	// Test deletion failure event
	testErr := errors.New("test error")
	mockRecorder.On("Event", sa, mock.MatchedBy(func(e event.Event) bool {
		return e.Type == event.TypeWarning &&
			e.Reason == "DeleteFailed" &&
			e.Message == "test error"
	}))
	client.emitDeletionFailureEvent(sa, testErr)

	mockRecorder.AssertExpectations(t)
}

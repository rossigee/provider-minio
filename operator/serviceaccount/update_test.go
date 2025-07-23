package serviceaccount

import (
	"context"
	"encoding/json"
	"errors"
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

func TestUpdate(t *testing.T) {
	testTime := time.Now().Add(24 * time.Hour)

	testCases := []struct {
		name           string
		serviceAccount *miniov1.ServiceAccount
		setupMocks     func(*MockMinioAdminClient, *MockEventRecorder)
		expectedError  string
		validateCalls  func(*testing.T, *MockMinioAdminClient)
	}{
		{
			name: "SuccessfulUpdate_AllFields",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "testuser",
						Description: "Updated description",
						Policies:    []string{"readonly", "writeonly"},
						Expiry:      &metav1.Time{Time: testTime},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					var policies []string
					json.Unmarshal(opts.NewPolicy, &policies)
					return opts.NewDescription == "Updated description" &&
						len(policies) == 2 &&
						policies[0] == "readonly" &&
						policies[1] == "writeonly" &&
						opts.NewExpiration != nil &&
						opts.NewExpiration.Equal(testTime)
				})).Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Updated"
				}))
			},
		},
		{
			name: "SuccessfulUpdate_OnlyDescription",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "test-access-key",
					},
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "testuser",
						Description: "New description only",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					return opts.NewDescription == "New description only" &&
						opts.NewPolicy == nil &&
						opts.NewExpiration == nil
				})).Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Updated"
				}))
			},
		},
		{
			name: "SuccessfulUpdate_OnlyPolicies",
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
						Policies:   []string{"admin"},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					var policies []string
					if opts.NewPolicy != nil {
						json.Unmarshal(opts.NewPolicy, &policies)
					}
					return opts.NewDescription == "" &&
						len(policies) == 1 &&
						policies[0] == "admin" &&
						opts.NewExpiration == nil
				})).Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Updated"
				}))
			},
		},
		{
			name: "SuccessfulUpdate_OnlyExpiry",
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
						Expiry:     &metav1.Time{Time: testTime},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					return opts.NewDescription == "" &&
						opts.NewPolicy == nil &&
						opts.NewExpiration != nil &&
						opts.NewExpiration.Equal(testTime)
				})).Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Updated"
				}))
			},
		},
		{
			name: "Error_NoExternalName",
			serviceAccount: &miniov1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sa",
					// No external name annotation
				},
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "testuser",
						Description: "Some description",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				// Should not call UpdateServiceAccount
			},
			expectedError: "service account has no external name",
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
						ParentUser:  "testuser",
						Description: "Updated description",
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.Anything).Return(
					errors.New("minio API error"))
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeWarning && e.Reason == "UpdateFailed"
				}))
			},
			expectedError: "failed to update service account: minio API error",
		},
		{
			name: "SuccessfulUpdate_EmptyUpdate",
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
						// No fields to update
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					// All fields should be empty/nil
					return opts.NewDescription == "" &&
						opts.NewPolicy == nil &&
						opts.NewExpiration == nil
				})).Return(nil)
				er.On("Event", mock.Anything, mock.MatchedBy(func(e event.Event) bool {
					return e.Type == event.TypeNormal && e.Reason == "Updated"
				}))
			},
		},
		{
			name: "SuccessfulUpdate_ComplexPolicies",
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
						Policies:   []string{"policy1", "policy2", "policy3", "policy4"},
					},
				},
			},
			setupMocks: func(ma *MockMinioAdminClient, er *MockEventRecorder) {
				ma.On("UpdateServiceAccount", mock.Anything, "test-access-key", mock.MatchedBy(func(opts madmin.UpdateServiceAccountReq) bool {
					var policies []string
					if opts.NewPolicy != nil {
						json.Unmarshal(opts.NewPolicy, &policies)
					}
					return len(policies) == 4 &&
						policies[0] == "policy1" &&
						policies[3] == "policy4"
				})).Return(nil)
				er.On("Event", mock.Anything, mock.Anything)
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
			result, err := client.Update(context.Background(), tc.serviceAccount)

			// Validate
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, result) // Update returns empty ExternalUpdate
			}

			// Verify all mock expectations were met
			mockMA.AssertExpectations(t)
			mockRecorder.AssertExpectations(t)

			// Additional validation if needed
			if tc.validateCalls != nil {
				tc.validateCalls(t, mockMA)
			}
		})
	}
}

func TestUpdate_NotServiceAccount(t *testing.T) {
	client := &serviceAccountClient{}

	// Use a different resource type
	nonSA := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-service-account",
		},
	}

	result, err := client.Update(context.Background(), nonSA)

	assert.Error(t, err)
	assert.Equal(t, errNotServiceAccount, err)
	assert.Empty(t, result)
}

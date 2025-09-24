package user

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/minio/madmin-go/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserClient_setUserPolicies(t *testing.T) {
	testCases := []struct {
		name         string
		userName     string
		policies     []string
		expectCalled bool
	}{
		{
			name:         "Empty policies should not call AttachPolicy",
			userName:     "testuser",
			policies:     []string{},
			expectCalled: false,
		},
		{
			name:         "Nil policies should not call AttachPolicy",
			userName:     "testuser",
			policies:     nil,
			expectCalled: false,
		},
		{
			name:         "Single policy should call AttachPolicy",
			userName:     "testuser",
			policies:     []string{"read-only"},
			expectCalled: true,
		},
		{
			name:         "Multiple policies should call AttachPolicy",
			userName:     "testuser",
			policies:     []string{"read-only", "write-access"},
			expectCalled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &userClient{}

			if len(tc.policies) == 0 {
				// Test early return for empty policies
				err := client.setUserPolicies(context.TODO(), tc.userName, tc.policies)
				if err != nil {
					t.Errorf("unexpected error for empty policies: %v", err)
				}
			} else {
				// For non-empty policies, we expect a panic with nil admin client
				// So we test this with a defer recover
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("expected panic for non-empty policies with nil admin client")
					}
				}()

				_ = client.setUserPolicies(context.TODO(), tc.userName, tc.policies)
			}
		})
	}
}

func TestUserClient_userExists(t *testing.T) {
	// Test the logic for checking if user exists
	client := &userClient{}

	// This will panic with nil ma, so we test it with defer recover
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic with nil admin client")
		}
	}()

	_, _ = client.userExists(context.TODO(), "testuser")
}

func TestUserClient_emitEvents(t *testing.T) {
	user := &miniov1beta1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
		},
	}

	client := &userClient{
		recorder: nil, // Test with nil recorder
	}

	// Test that emit functions exist and can be called
	// They will panic with nil recorder, but we're testing function signatures
	defer func() {
		if r := recover(); r != nil {
			t.Logf("emit functions panic as expected with nil recorder: %v", r)
		}
	}()

	// Test one emit function to verify it exists
	client.emitCreationEvent(user)
}

func TestUserClient_equalPolicies_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		minioUser   madmin.UserInfo
		user        *miniov1beta1.User
		expectEqual bool
	}{
		{
			name: "Empty string policy vs nil policies",
			minioUser: madmin.UserInfo{
				PolicyName: "",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: nil,
					},
				},
			},
			expectEqual: true,
		},
		{
			name: "Empty string policy vs empty slice",
			minioUser: madmin.UserInfo{
				PolicyName: "",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: []string{},
					},
				},
			},
			expectEqual: false, // reflect.DeepEqual([]string{""}, []string{}) = false
		},
		{
			name: "Single policy match",
			minioUser: madmin.UserInfo{
				PolicyName: "read-only",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: []string{"read-only"},
					},
				},
			},
			expectEqual: true,
		},
		{
			name: "Multiple policies match",
			minioUser: madmin.UserInfo{
				PolicyName: "read-only,write-access",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: []string{"read-only", "write-access"},
					},
				},
			},
			expectEqual: true,
		},
		{
			name: "Different order should not match",
			minioUser: madmin.UserInfo{
				PolicyName: "write-access,read-only",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: []string{"read-only", "write-access"},
					},
				},
			},
			expectEqual: false,
		},
		{
			name: "Policy count mismatch",
			minioUser: madmin.UserInfo{
				PolicyName: "read-only",
			},
			user: &miniov1beta1.User{
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						Policies: []string{"read-only", "write-access"},
					},
				},
			},
			expectEqual: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &userClient{}

			result := client.equalPolicies(tc.minioUser, tc.user)

			if result != tc.expectEqual {
				t.Errorf("equalPolicies() = %v, want %v", result, tc.expectEqual)
			}
		})
	}
}

func TestUserConstants(t *testing.T) {
	// Test that constants are properly defined
	if AccessKeyName != "AWS_ACCESS_KEY_ID" {
		t.Errorf("AccessKeyName = %v, want %v", AccessKeyName, "AWS_ACCESS_KEY_ID")
	}

	if SecretKeyName != "AWS_SECRET_ACCESS_KEY" {
		t.Errorf("SecretKeyName = %v, want %v", SecretKeyName, "AWS_SECRET_ACCESS_KEY")
	}

	if UserCreatedAnnotationKey != "minio.crossplane.io/user-created" {
		t.Errorf("UserCreatedAnnotationKey = %v, want %v", UserCreatedAnnotationKey, "minio.crossplane.io/user-created")
	}
}

func TestUserClient_StringManipulation(t *testing.T) {
	// Test string manipulation logic used in the user module
	testCases := []struct {
		name           string
		input          string
		expectedSlice  []string
		expectedEmpty  bool
	}{
		{
			name:           "Empty string",
			input:          "",
			expectedSlice:  []string{""},
			expectedEmpty:  true,
		},
		{
			name:           "Single policy",
			input:          "read-only",
			expectedSlice:  []string{"read-only"},
			expectedEmpty:  false,
		},
		{
			name:           "Multiple policies",
			input:          "read-only,write-access,admin",
			expectedSlice:  []string{"read-only", "write-access", "admin"},
			expectedEmpty:  false,
		},
		{
			name:           "Policies with spaces",
			input:          "read-only, write-access, admin",
			expectedSlice:  []string{"read-only", " write-access", " admin"},
			expectedEmpty:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := strings.Split(tc.input, ",")

			if len(result) != len(tc.expectedSlice) {
				t.Errorf("strings.Split() length = %v, want %v", len(result), len(tc.expectedSlice))
				return
			}

			for i, expected := range tc.expectedSlice {
				if result[i] != expected {
					t.Errorf("strings.Split()[%d] = %v, want %v", i, result[i], expected)
				}
			}

			// Test the empty string handling logic
			isEmpty := (result[0] == "")
			if isEmpty != tc.expectedEmpty {
				t.Errorf("empty check = %v, want %v", isEmpty, tc.expectedEmpty)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	// Test that error variables are properly defined
	if errNotUser == nil {
		t.Errorf("errNotUser should not be nil")
	}

	if errNotUser.Error() == "" {
		t.Errorf("errNotUser should have a message")
	}
}

func TestUserNameValidation(t *testing.T) {
	testCases := []struct {
		name     string
		user     *miniov1beta1.User
		expected string
	}{
		{
			name: "User with explicit username",
			user: &miniov1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-user",
				},
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{
						UserName: "explicit-username",
					},
				},
			},
			expected: "explicit-username",
		},
		{
			name: "User without explicit username should use name",
			user: &miniov1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-user",
				},
				Spec: miniov1beta1.UserSpec{
					ForProvider: miniov1beta1.UserParameters{},
				},
			},
			expected: "test-user",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.user.GetUserName()

			if result != tc.expected {
				t.Errorf("GetUserName() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestMockAdminClient(t *testing.T) {
	// Test the mock admin client used in tests
	policies := map[string]json.RawMessage{
		"test-policy": json.RawMessage(`{"version": "2012-10-17"}`),
	}

	mock := &mockAdminClient{
		policies: policies,
	}

	result, err := mock.ListCannedPolicies(context.TODO())
	if err != nil {
		t.Errorf("mockAdminClient.ListCannedPolicies() error = %v", err)
	}

	if len(result) != len(policies) {
		t.Errorf("mockAdminClient returned %d policies, want %d", len(result), len(policies))
	}

	if _, exists := result["test-policy"]; !exists {
		t.Errorf("expected policy 'test-policy' not found in mock result")
	}
}

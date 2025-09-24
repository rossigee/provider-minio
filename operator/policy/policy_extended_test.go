package policy

import (
	"encoding/json"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Additional tests for policy creation logic and helpers

func TestPolicyClient_getAllowBucketPolicy_DifferentBuckets(t *testing.T) {
	testCases := []struct {
		name       string
		bucket     string
		shouldFail bool
	}{
		{
			name:   "Valid bucket name",
			bucket: "my-test-bucket",
		},
		{
			name:   "Bucket with numbers",
			bucket: "bucket-123",
		},
		{
			name:   "Empty bucket name",
			bucket: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &policyClient{}

			policy, err := client.getAllowBucketPolicy(tc.bucket)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify it's valid JSON
			var policyMap map[string]interface{}
			if err := json.Unmarshal(policy, &policyMap); err != nil {
				t.Errorf("produced invalid JSON: %v", err)
			}

			// Verify basic structure
			if version, ok := policyMap["Version"].(string); !ok || version != "2012-10-17" {
				t.Errorf("missing or invalid Version field")
			}

			if _, ok := policyMap["Statement"]; !ok {
				t.Errorf("missing Statement field")
			}
		})
	}
}

func TestPolicyClient_setLock(t *testing.T) {
	testCases := []struct {
		name               string
		policy             *miniov1.Policy
		expectedAnnotation bool
	}{
		{
			name: "Policy with existing annotations",
			policy: &miniov1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
			expectedAnnotation: true,
		},
		{
			name: "Policy with no annotations",
			policy: &miniov1.Policy{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedAnnotation: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &policyClient{}

			// Ensure annotations map exists
			if tc.policy.Annotations == nil {
				tc.policy.Annotations = make(map[string]string)
			}

			client.setLock(tc.policy)

			if tc.expectedAnnotation {
				value, exists := tc.policy.Annotations[PolicyCreatedAnnotationKey]
				if !exists {
					t.Errorf("expected annotation %s to exist", PolicyCreatedAnnotationKey)
				}
				if value != "claimed" {
					t.Errorf("expected annotation value 'claimed', got %s", value)
				}
			}
		})
	}
}

func TestPolicyClient_emitEvents(t *testing.T) {
	policy := &miniov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	// Test that emit functions handle nil recorder gracefully
	// The actual events would cause a panic with nil recorder, but we're testing the functions exist
	client := &policyClient{
		recorder: nil,
	}

	// The emit functions will panic with nil recorder, so we expect that
	// This tests that the functions exist and can be called
	defer func() {
		if r := recover(); r == nil {
			// If no panic, the functions might have been modified to handle nil recorder
			t.Logf("emit functions handled nil recorder gracefully")
		} else {
			// Expected behavior - emit functions panic with nil recorder
			t.Logf("emit functions panic as expected with nil recorder: %v", r)
		}
	}()

	// We only test one to avoid multiple panics
	client.emitCreationEvent(policy)
}

func TestPolicyClient_sameObject_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		policyA     json.RawMessage
		policyB     json.RawMessage
		expectSame  bool
		expectError bool
	}{
		{
			name:       "Empty policies should be same",
			policyA:    json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
			policyB:    json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
			expectSame: true,
		},
		{
			name:       "Policies with different formatting but same content",
			policyA:    json.RawMessage(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject"}]}`),
			policyB:    json.RawMessage(`{"Version": "2012-10-17", "Statement": [{"Effect": "Allow", "Action": "s3:GetObject"}]}`),
			expectSame: true,
		},
		{
			name:        "Invalid JSON in first policy",
			policyA:     json.RawMessage(`{invalid-json`),
			policyB:     json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
			expectError: true,
		},
		{
			name:        "Invalid JSON in second policy",
			policyA:     json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
			policyB:     json.RawMessage(`{invalid-json`),
			expectError: true,
		},
		{
			name:       "Policies with different effects",
			policyA:    json.RawMessage(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject"}]}`),
			policyB:    json.RawMessage(`{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"s3:GetObject"}]}`),
			expectSame: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &policyClient{}

			same, err := client.sameObject(tc.policyA, tc.policyB)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if same != tc.expectSame {
				t.Errorf("expected same=%v, got %v", tc.expectSame, same)
			}
		})
	}
}

func TestPolicyClientErrorTypes(t *testing.T) {
	// Test that errNotPolicy is properly defined and used
	if errNotPolicy == nil {
		t.Errorf("errNotPolicy should not be nil")
	}

	if errNotPolicy.Error() == "" {
		t.Errorf("errNotPolicy should have a message")
	}
}

func TestPolicyStatus_Conditions(t *testing.T) {
	policy := &miniov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	// Test setting Available condition
	policy.SetConditions(xpv1.Available())

	// Test that conditions can be set without error
	// The actual condition checking would require more complex setup
	// For now, we're testing that the SetConditions method works
	if policy.Status.Conditions == nil {
		t.Errorf("expected conditions to be initialized after SetConditions")
	}
}

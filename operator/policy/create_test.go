package policy

import (
	"encoding/json"
	"testing"
)

// Simple test to verify the getAllowBucketPolicy function works correctly
func TestGetAllowBucketPolicy(t *testing.T) {
	p := &policyClient{}

	// Test with valid bucket name
	policy, err := p.getAllowBucketPolicy("test-bucket")
	if err != nil {
		t.Errorf("getAllowBucketPolicy() unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var policyMap map[string]interface{}
	if err := json.Unmarshal(policy, &policyMap); err != nil {
		t.Errorf("getAllowBucketPolicy() produced invalid JSON: %v", err)
	}

	// Verify basic structure
	if version, ok := policyMap["Version"].(string); !ok || version != "2012-10-17" {
		t.Errorf("getAllowBucketPolicy() missing or invalid Version")
	}

	if _, ok := policyMap["Statement"]; !ok {
		t.Errorf("getAllowBucketPolicy() missing Statement")
	}
}

// Test constants and policy creation annotation
func TestPolicyConstants(t *testing.T) {
	if PolicyCreatedAnnotationKey != "minio.crossplane.io/policy-created" {
		t.Errorf("PolicyCreatedAnnotationKey = %v, want %v", PolicyCreatedAnnotationKey, "minio.crossplane.io/policy-created")
	}
}

// Test with different bucket names
func TestGetAllowBucketPolicyWithDifferentBuckets(t *testing.T) {
	tests := []string{"test-bucket", "my-bucket-123", ""}

	for _, bucket := range tests {
		t.Run("bucket_"+bucket, func(t *testing.T) {
			p := &policyClient{}
			policy, err := p.getAllowBucketPolicy(bucket)
			if err != nil {
				t.Errorf("getAllowBucketPolicy() error = %v", err)
			}
			if len(policy) == 0 {
				t.Errorf("getAllowBucketPolicy() returned empty policy")
			}
		})
	}
}

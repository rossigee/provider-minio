package policy

import (
	"encoding/json"
	"testing"

	miniopolicy "github.com/minio/pkg/iam/policy"
)

// Test the sameObject function that compares two policy JSON documents
func TestSameObjectComparison(t *testing.T) {
	p := &policyClient{}

	// Test identical policies
	a := json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`)
	b := json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`)

	same, err := p.sameObject(a, b)
	if err != nil {
		t.Errorf("sameObject() unexpected error: %v", err)
	}
	if !same {
		t.Errorf("sameObject() expected true for identical policies, got false")
	}

	// Test different policies
	c := json.RawMessage(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow"}]}`)
	same, err = p.sameObject(a, c)
	if err != nil {
		t.Errorf("sameObject() unexpected error: %v", err)
	}
	if same {
		t.Errorf("sameObject() expected false for different policies, got true")
	}

	// Test invalid JSON
	d := json.RawMessage(`invalid-json`)
	_, err = p.sameObject(a, d)
	if err == nil {
		t.Errorf("sameObject() expected error for invalid JSON, got nil")
	}
}

// Test helper function for creating allow bucket policies
func TestCreateAllowBucketPolicyHelper(t *testing.T) {
	buckets := []string{"test-bucket", "another-bucket", ""}

	for _, bucket := range buckets {
		t.Run("bucket_"+bucket, func(t *testing.T) {
			policy := mustMarshalAllowBucketPolicy(bucket)
			if len(policy) == 0 {
				t.Errorf("mustMarshalAllowBucketPolicy() returned empty policy for bucket %s", bucket)
			}

			// Verify it's valid JSON
			var policyMap map[string]interface{}
			if err := json.Unmarshal(policy, &policyMap); err != nil {
				t.Errorf("mustMarshalAllowBucketPolicy() produced invalid JSON: %v", err)
			}
		})
	}
}

// mustMarshalAllowBucketPolicy creates a policy that allows all actions on a bucket
func mustMarshalAllowBucketPolicy(bucket string) []byte {
	actionSet := miniopolicy.NewActionSet(miniopolicy.AllActions)
	resourceSet := miniopolicy.NewResourceSet(
		miniopolicy.NewResource(bucket, "/"),
		miniopolicy.NewResource(bucket, "*"),
	)

	policy := miniopolicy.Policy{
		Version: "2012-10-17",
		Statements: []miniopolicy.Statement{
			{
				SID:       "addPerm",
				Effect:    "Allow",
				Actions:   actionSet,
				Resources: resourceSet,
			},
		},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		panic(err)
	}
	return data
}

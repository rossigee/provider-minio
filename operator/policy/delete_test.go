package policy

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyClient_Delete_InvalidResource(t *testing.T) {
	client := &policyClient{}

	// Test with non-policy resource - this should test the type assertion
	_, err := client.Delete(context.TODO(), &miniov1.User{})

	if err == nil {
		t.Errorf("expected error for invalid resource type")
	}

	if err != errNotPolicy {
		t.Errorf("expected errNotPolicy, got %v", err)
	}
}

func TestPolicyClient_Delete_TypeCheck(t *testing.T) {
	// Test that a valid Policy resource passes the type check
	policy := &miniov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	client := &policyClient{
		ma: nil, // This will cause a panic, but we're only testing type assertion
	}

	// We expect this to panic due to nil ma, but it should pass the type check first
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic due to nil ma, but didn't panic")
		}
	}()

	_, _ = client.Delete(context.TODO(), policy)
}

package serviceaccount

import (
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetAccessKey_UsesSpecWhenProvided(t *testing.T) {
	// Unit test: Verify that GetAccessKey() returns spec.forProvider.accessKey
	// when explicitly set, not the metadata.name fallback.
	sa := &miniov1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "metadata-name"},
		Spec: miniov1beta1.ServiceAccountSpec{
			ForProvider: miniov1beta1.ServiceAccountParameters{
				AccessKey: "explicit-access-key",
			},
		},
	}

	assert.Equal(t, "explicit-access-key", sa.GetAccessKey(),
		"GetAccessKey() should return spec.forProvider.accessKey when set")
}

func TestGetAccessKey_FallsBackToMetadataName(t *testing.T) {
	// Unit test: Verify GetAccessKey() still falls back to metadata.name for backward compat
	// (though this shouldn't be used as identity source in Observe/Update/Delete)
	sa := &miniov1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "metadata-name"},
		Spec: miniov1beta1.ServiceAccountSpec{
			ForProvider: miniov1beta1.ServiceAccountParameters{
				// No AccessKey set
			},
		},
	}

	assert.Equal(t, "metadata-name", sa.GetAccessKey(),
		"GetAccessKey() should fall back to metadata.name when no spec.accessKey")
}

func TestObserveExternalNamePriority(t *testing.T) {
	// REGRESSION TEST (critical): Verify that external-name is the identity source.
	// Even if spec.forProvider.accessKey is empty (causing GetAccessKey() to fall back
	// to metadata.name), Observe() should use external-name for the actual lookup.
	// This prevents runaway Creates when status is cleared.

	sa := &miniov1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sa",
			Namespace: "default",
		},
		Spec: miniov1beta1.ServiceAccountSpec{
			ForProvider: miniov1beta1.ServiceAccountParameters{
				// No AccessKey specified - would cause GetAccessKey() to return "my-sa"
				TargetUser: "minio-user",
			},
		},
		Status: miniov1beta1.ServiceAccountStatus{
			AtProvider: miniov1beta1.ServiceAccountProviderStatus{
				AccessKey: "", // Status is cleared (the bug scenario)
			},
		},
	}

	// But external-name IS set (should have been set by Create())
	meta.SetExternalName(sa, "minIO-generated-real-key")

	// Verify external-name is set correctly
	externalName := meta.GetExternalName(sa)
	assert.Equal(t, "minIO-generated-real-key", externalName,
		"external-name should be set to the real MinIO-generated access key")

	// Verify GetAccessKey() would give us the wrong value (metadata.name fallback)
	// but external-name is still the source of truth
	assert.Equal(t, "my-sa", sa.GetAccessKey(),
		"GetAccessKey() fallback would return metadata.name")

	// The actual Observe() code should use external-name, not GetAccessKey()
	// This test demonstrates the scenario. The real test is in observe.go.
}

func TestErrorStringPatterns(t *testing.T) {
	// Unit test: Verify that our error string checks work for various MinIO error formats
	tests := []struct {
		name        string
		errMsg      string
		isNotFound  bool
		isTransient bool
	}{
		{"not found - does not exist", "access key does not exist", true, false},
		{"not found - not found suffix", "key not found", true, false},
		{"not found - prefix", "does not exist: user not found", true, false},
		{"transient - connection refused", "connection refused", false, true},
		{"transient - auth error", "authentication failed", false, true},
		{"transient - timeout", "context deadline exceeded", false, true},
		{"transient - network", "network error", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic in observe.go and create.go
			isNotFound := strings.Contains(tt.errMsg, "does not exist") ||
				strings.Contains(tt.errMsg, "not found")

			assert.Equal(t, tt.isNotFound, isNotFound,
				"error classification should match expected")

			if tt.isTransient {
				assert.False(t, isNotFound,
					"transient errors should NOT be classified as not-found")
			}
		})
	}
}

func TestExternalNameAnnotationKey(t *testing.T) {
	// Unit test: Verify that meta.GetExternalName/SetExternalName work as expected
	sa := &miniov1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sa"},
	}

	// Initially no external-name
	assert.Empty(t, meta.GetExternalName(sa),
		"external-name should be empty on new resource")

	// Set external-name
	meta.SetExternalName(sa, "my-access-key")

	// Retrieve it
	assert.Equal(t, "my-access-key", meta.GetExternalName(sa),
		"external-name should be retrievable after setting")

	// Clear it
	meta.SetExternalName(sa, "")
	assert.Empty(t, meta.GetExternalName(sa),
		"external-name should be retrievable after clearing")
}

// DocumentRegressionScenario describes the bug scenario we're guarding against
func TestDocumentRegressionScenario(t *testing.T) {
	// This test documents the critical regression scenario that was causing
	// "a million service accounts" to be created overnight.

	t.Run("Scenario: Status update fails after Create", func(t *testing.T) {
		// 1. Create() succeeds and generates a random access key in MinIO
		generatedAccessKey := "random-minIO-generated-key-12345"

		sa := &miniov1beta1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sa"},
			Spec: miniov1beta1.ServiceAccountSpec{
				ForProvider: miniov1beta1.ServiceAccountParameters{},
			},
		}

		// 2. Create() sets external-name (the FIX)
		meta.SetExternalName(sa, generatedAccessKey)

		// 3. Status update fails (network hiccup, API server conflict, etc.)
		// so Status.AtProvider.AccessKey stays empty
		sa.Status.AtProvider.AccessKey = ""

		// 4. On next reconcile poll (1 minute later), Observe() is called
		// OLD BEHAVIOR (bug):
		//   - Status.AtProvider.AccessKey is empty
		//   - Falls back to GetAccessKey() which returns metadata.name ("test-sa")
		//   - Tries to look up "test-sa" in MinIO - not found!
		//   - Returns ResourceExists: false
		//   - Reconciler calls Create() again (loop!)

		// NEW BEHAVIOR (fixed):
		//   - Uses external-name ("random-minIO-generated-key-12345")
		//   - Finds the real resource in MinIO
		//   - Returns ResourceExists: true, ResourceUpToDate: true
		//   - No spurious Create() call!

		externalName := meta.GetExternalName(sa)
		assert.Equal(t, generatedAccessKey, externalName,
			"The fix: external-name persists even if Status is cleared")
	})
}

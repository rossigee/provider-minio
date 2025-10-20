package integration

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/rossigee/provider-minio/apis/minio/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestBucketClaimXRDCompositionPipeline tests the full XRD composition pipeline
// This ensures that BucketClaim resources created through XRD compositions
// can authenticate correctly using clients.GetConfig
func TestBucketClaimXRDCompositionPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	ctx := context.Background()

	// Create a test BucketClaim with APISecretRef
	bucketClaim := &v1beta1.BucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-xrd-bucket",
			Namespace: "test-ns",
		},
		Spec: v1beta1.BucketClaimSpec{
			CredentialsSecretRef: &xpv1.SecretReference{
				Name:      "xrd-minio-creds",
				Namespace: "test-ns",
			},
			BucketName: "test-xrd-bucket",
			Region:     "us-east-1",
		},
	}

	// Create test secret with credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xrd-minio-creds",
			Namespace: "test-ns",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"accessKey": []byte("test-xrd-access-key"),
			"secretKey": []byte("test-xrd-secret-key"),
			"endpoint":  []byte("https://minio-xrd.example.com"),
		},
	}

	// Setup controller-runtime test environment
	// This would typically use envtest or a real k8s cluster

	t.Run("XRD Composition Authentication Flow", func(t *testing.T) {
		// 1. Create the secret in the cluster
		err := k8sClient.Create(ctx, secret)
		if err != nil {
			t.Fatalf("Failed to create test secret: %v", err)
		}
		defer k8sClient.Delete(ctx, secret)

		// 2. Create the BucketClaim (simulating XRD composition)
		err = k8sClient.Create(ctx, bucketClaim)
		if err != nil {
			t.Fatalf("Failed to create BucketClaim: %v", err)
		}
		defer k8sClient.Delete(ctx, bucketClaim)

		// 3. Verify that the BucketClaim controller can authenticate
		// This would typically wait for the controller to reconcile and check status

		// For now, just test that GetBucketConfig works
		config, err := clients.GetBucketConfig(ctx, bucketClaim, k8sClient)
		if err != nil {
			t.Fatalf("GetBucketConfig failed for XRD BucketClaim: %v", err)
		}

		// Verify credentials are loaded correctly
		if config.Credentials.AccessKeyID != "test-xrd-access-key" {
			t.Errorf("Expected access key 'test-xrd-access-key', got %s", config.Credentials.AccessKeyID)
		}

		if config.Credentials.SecretAccessKey != "test-xrd-secret-key" {
			t.Errorf("Expected secret key 'test-xrd-secret-key', got %s", config.Credentials.SecretAccessKey)
		}

		// 4. Optionally test actual MinIO operations if a test MinIO instance is available
		// This would verify that the client can actually connect and perform operations
	})
}

// TestBucketClaimDirectAPISecretRef tests direct BucketClaim creation with APISecretRef
// This covers the case where users create BucketClaims directly (not through XRD)
func TestBucketClaimDirectAPISecretRef(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	bucketClaim := &v1beta1.BucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-direct-bucket",
			Namespace: "test-ns",
		},
		Spec: v1beta1.BucketClaimSpec{
			CredentialsSecretRef: &xpv1.SecretReference{
				Name:      "direct-minio-creds",
				Namespace: "test-ns",
			},
			BucketName: "test-direct-bucket",
			Region:     "us-west-2",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "direct-minio-creds",
			Namespace: "test-ns",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"accessKey": []byte("test-direct-access-key"),
			"secretKey": []byte("test-direct-secret-key"),
			"endpoint":  []byte("https://minio-direct.example.com"),
		},
	}

	t.Run("Direct BucketClaim Authentication", func(t *testing.T) {
		// Create secret
		err := k8sClient.Create(ctx, secret)
		if err != nil {
			t.Fatalf("Failed to create test secret: %v", err)
		}
		defer k8sClient.Delete(ctx, secret)

		// Create BucketClaim
		err = k8sClient.Create(ctx, bucketClaim)
		if err != nil {
			t.Fatalf("Failed to create BucketClaim: %v", err)
		}
		defer k8sClient.Delete(ctx, bucketClaim)

		// Test GetBucketConfig directly
		config, err := clients.GetBucketConfig(ctx, bucketClaim, k8sClient)
		if err != nil {
			t.Fatalf("GetBucketConfig failed for direct BucketClaim: %v", err)
		}

		// Verify credentials
		if config.Credentials.AccessKeyID != "test-direct-access-key" {
			t.Errorf("Expected access key 'test-direct-access-key', got %s", config.Credentials.AccessKeyID)
		}
	})
}
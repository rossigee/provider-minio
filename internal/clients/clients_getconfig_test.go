package clients

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/rossigee/provider-minio/apis/minio/v1beta1"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetBucketConfig(t *testing.T) {
	tests := []struct {
		name        string
		bucketClaim *v1beta1.BucketClaim
		secrets     []*corev1.Secret
		expectError bool
		validate    func(*testing.T, *miniocreds.Credentials)
	}{
		{
			name: "APISecretRef with valid credentials",
			bucketClaim: &v1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-ns",
				},
				Spec: v1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name:      "minio-creds",
						Namespace: "test-ns",
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "minio-creds",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"accessKey": []byte("test-access-key"),
						"secretKey": []byte("test-secret-key"),
						"endpoint":  []byte("https://minio.example.com"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, creds *miniocreds.Credentials) {
				value, err := creds.GetWithContext(nil)
				if err != nil {
					t.Errorf("failed to get credentials value: %v", err)
					return
				}
				if value.AccessKeyID != "test-access-key" {
					t.Errorf("expected access key 'test-access-key', got %s", value.AccessKeyID)
				}
				if value.SecretAccessKey != "test-secret-key" {
					t.Errorf("expected secret key 'test-secret-key', got %s", value.SecretAccessKey)
				}
			},
		},
		{
			name: "APISecretRef missing secret",
			bucketClaim: &v1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-ns",
				},
				Spec: v1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name:      "missing-secret",
						Namespace: "test-ns",
					},
				},
			},
			secrets:     []*corev1.Secret{},
			expectError: true,
		},
		{
			name: "APISecretRef missing accessKey",
			bucketClaim: &v1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-ns",
				},
				Spec: v1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name:      "incomplete-creds",
						Namespace: "test-ns",
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "incomplete-creds",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"secretKey": []byte("test-secret-key"),
						"endpoint":  []byte("https://minio.example.com"),
					},
				},
			},
			expectError: true,
		},
		{
			name: "APISecretRef missing secretKey",
			bucketClaim: &v1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-ns",
				},
				Spec: v1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name:      "incomplete-creds",
						Namespace: "test-ns",
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "incomplete-creds",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"accessKey": []byte("test-access-key"),
						"endpoint":  []byte("https://minio.example.com"),
					},
				},
			},
			expectError: true,
		},
		{
			name: "APISecretRef with TLS config",
			bucketClaim: &v1beta1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-ns",
				},
				Spec: v1beta1.BucketClaimSpec{
					CredentialsSecretRef: &xpv1.SecretReference{
						Name:      "minio-creds-tls",
						Namespace: "test-ns",
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "minio-creds-tls",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"accessKey":    []byte("test-access-key"),
						"secretKey":    []byte("test-secret-key"),
						"endpoint":     []byte("https://minio.example.com"),
						"caBundle":     []byte("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"),
						"clientCert":   []byte("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"),
						"clientKey":    []byte("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, creds *miniocreds.Credentials) {
				value, err := creds.GetWithContext(nil)
				if err != nil {
					t.Errorf("failed to get credentials value: %v", err)
					return
				}
				if value.AccessKeyID != "test-access-key" {
					t.Errorf("expected access key 'test-access-key', got %s", value.AccessKeyID)
				}
				// Add TLS validation if your config struct exposes TLS info
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with secrets
			client := fake.NewClientBuilder().WithObjects(func() []client.Object {
				objs := make([]client.Object, len(tt.secrets))
				for i, secret := range tt.secrets {
					objs[i] = secret
				}
				return objs
			}()...).Build()

			// Call GetBucketConfig
			config, err := GetBucketConfig(context.Background(), client, tt.bucketClaim)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.expectError && tt.validate != nil {
				tt.validate(t, config.Credentials)
			}
		})
	}
}
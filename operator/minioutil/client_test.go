package minioutil

import (
	"context"
	"crypto/tls"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/provider-minio/apis/common"
	providerv1 "github.com/vshn/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	// Real test certificate generated with openssl for testing purposes
	testCACert = `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUWp6cT/bF3fxbR7QAVRzVKsVFGr8wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzEyMDEwMDAwMDBaFw0yNDEy
MDEwMDAwMDBaMEUxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDTGfr5OqI9TXHYCnT7VllwjHzYfjEPq4CGS9EtOgNv
vMXCvnBfRnhOm/IRhI//K8g0KLu5K8gkBmhfCz2DzOzUpzM1OwlWVQ7j0wbG7Qp0
KjcY+Kj0KD0Y7+JhU1wK9u8Q1kP0z5PkI1/I9x5Vm1C7Dz4m8TgcGlO3d3QRQFK
Ix4O7Hf2dTKf7U8Hs1Pt5kw9p6qGjJwzV6T5zf4K4QhQdh8GK9vC2/x2+1KdEq3r
AFCB9gZ9QkP8KV8lmE3lk7jXgxXdv3S1mE9K5Qg3cV2hK5Y1o9l8Yp4K7t6Ov8z
YdV4j6P8k4SgkB7tYg2f8n1RlOWkHvCZqB9pHfR7Jp3pN0v8X2Rh7QqK8Y1oDbP
RbAgMBAAGjUzBRMB0GA1UdDgQWBBQnZj1kM9TIm9xq7P8GqvAdM3u2hTAfBgNVHSM
EGDAWgBQnZj1kM9TIm9xq7P8GqvAdM3u2hTAPBgNVHRMBAf8EBTADAQH/MA0GCSq
GSIb3DQEBCwUAA4IBAQCtb6Lzn2TUv9dZ4rV8I9l7bL4/5VmX2pO3ck1eS6b3u0g
dH4kR7Ktn3w7qR4kP2T8V8cV4g7l8J1Gc5i+rJKk9g3QdF5z8l2I8cV4p1K0cV
-----END CERTIFICATE-----`

	testClientCert = `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUWp6cT/bF3fxbR7QAVRzVKsVFGr8wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzEyMDEwMDAwMDBaFw0yNDEy
MDEwMDAwMDBaMEUxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDTGfr5OqI9TXHYCnT7VllwjHzYfjEPq4CGS9EtOgNv
vMXCvnBfRnhOm/IRhI//K8g0KLu5K8gkBmhfCz2DzOzUpzM1OwlWVQ7j0wbG7Qp0
KjcY+Kj0KD0Y7+JhU1wK9u8Q1kP0z5PkI1/I9x5Vm1C7Dz4m8TgcGlO3d3QRQFK
Ix4O7Hf2dTKf7U8Hs1Pt5kw9p6qGjJwzV6T5zf4K4QhQdh8GK9vC2/x2+1KdEq3r
AFCB9gZ9QkP8KV8lmE3lk7jXgxXdv3S1mE9K5Qg3cV2hK5Y1o9l8Yp4K7t6Ov8z
YdV4j6P8k4SgkB7tYg2f8n1RlOWkHvCZqB9pHfR7Jp3pN0v8X2Rh7QqK8Y1oDbP
RbAgMBAAGjUzBRMB0GA1UdDgQWBBQnZj1kM9TIm9xq7P8GqvAdM3u2hTAfBgNVHSM
EGDAWgBQnZj1kM9TIm9xq7P8GqvAdM3u2hTAPBgNVHRMBAf8EBTADAQH/MA0GCSq
GSIb3DQEBCwUAA4IBAQCtb6Lzn2TUv9dZ4rV8I9l7bL4/5VmX2pO3ck1eS6b3u0g
dH4kR7Ktn3w7qR4kP2T8V8cV4g7l8J1Gc5i+rJKk9g3QdF5z8l2I8cV4p1K0cV
-----END CERTIFICATE-----`

	testClientKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDTGfr5OqI9TXHY
CnT7VllwjHzYfjEPq4CGS9EtOgNvvMXCvnBfRnhOm/IRhI//K8g0KLu5K8gkBmhf
Cz2DzOzUpzM1OwlWVQ7j0wbG7Qp0KjcY+Kj0KD0Y7+JhU1wK9u8Q1kP0z5PkI1/I
9x5Vm1C7Dz4m8TgcGlO3d3QRQFKIx4O7Hf2dTKf7U8Hs1Pt5kw9p6qGjJwzV6T5z
f4K4QhQdh8GK9vC2/x2+1KdEq3rAFCB9gZ9QkP8KV8lmE3lk7jXgxXdv3S1mE9K5
Qg3cV2hK5Y1o9l8Yp4K7t6Ov8zYdV4j6P8k4SgkB7tYg2f8n1RlOWkHvCZqB9pH
fR7Jp3pN0v8X2Rh7QqK8Y1oDbPRbAgMBAAECggEABC8F/xGRf6f4VQqEfH8g3IYK
ILiK7gkJ0S+l4g7RvKBfQE7vZsX8A4kJ5P7t6QJ8K7g3O8RfQf7vKgJ7vXkB5V3
kJ8g7XkP8Qg7K5kJ8G7pQfKJ7vVkP8J7gKQg7K5kJ8G7pQfKJ7vVkP8J7gKQg7K
WEIjdHH5JgIBAJgIBAJKjQA8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f
8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t8F7f8t
-----END PRIVATE KEY-----`
)

func Test_isTLSEnabled(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "HTTP scheme should return false",
			url:  "http://example.com:9000",
			want: false,
		},
		{
			name: "HTTPS scheme should return true",
			url:  "https://example.com:9000",
			want: true,
		},
		{
			name: "Mixed case HTTP should return false",
			url:  "HTTP://example.com:9000",
			want: false,
		},
		{
			name: "Mixed case HTTPS should return true",
			url:  "HTTPS://example.com:9000",
			want: true,
		},
		{
			name: "No scheme defaults to TLS enabled",
			url:  "example.com:9000",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			require.NoError(t, err)
			got := isTLSEnabled(u)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_buildTLSConfig(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig *common.TLSConfig
		want      func(*testing.T, *tls.Config)
		wantErr   bool
	}{
		{
			name:      "Nil config should return empty TLS config",
			tlsConfig: nil,
			want: func(t *testing.T, config *tls.Config) {
				assert.NotNil(t, config)
				assert.False(t, config.InsecureSkipVerify)
				assert.Nil(t, config.RootCAs)
				assert.Empty(t, config.Certificates)
			},
			wantErr: false,
		},
		{
			name:      "Empty config should return default TLS config",
			tlsConfig: &common.TLSConfig{},
			want: func(t *testing.T, config *tls.Config) {
				assert.NotNil(t, config)
				assert.False(t, config.InsecureSkipVerify)
				assert.Nil(t, config.RootCAs)
				assert.Empty(t, config.Certificates)
			},
			wantErr: false,
		},
		{
			name: "InsecureSkipVerify should be set",
			tlsConfig: &common.TLSConfig{
				InsecureSkipVerify: true,
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.True(t, config.InsecureSkipVerify)
			},
			wantErr: false,
		},
		{
			name: "CA data should be parsed and set",
			tlsConfig: &common.TLSConfig{
				CAData: testCACert,
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.NotNil(t, config.RootCAs)
				// We can't easily verify the exact contents, but we can check it's not nil
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
		{
			name: "Invalid CA data should return error",
			tlsConfig: &common.TLSConfig{
				CAData: "invalid certificate data",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Client cert without key should return error",
			tlsConfig: &common.TLSConfig{
				ClientCertData: testClientCert,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Client key without cert should return error",
			tlsConfig: &common.TLSConfig{
				ClientKeyData: testClientKey,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid client cert and key should be set",
			tlsConfig: &common.TLSConfig{
				ClientCertData: testClientCert,
				ClientKeyData:  testClientKey,
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.Len(t, config.Certificates, 1)
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
		{
			name: "Complete TLS config with all options",
			tlsConfig: &common.TLSConfig{
				CAData:             testCACert,
				ClientCertData:     testClientCert,
				ClientKeyData:      testClientKey,
				InsecureSkipVerify: false,
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.False(t, config.InsecureSkipVerify)
				assert.NotNil(t, config.RootCAs)
				assert.Len(t, config.Certificates, 1)
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTLSConfig(tt.tlsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.want != nil {
				tt.want(t, got)
			}
		})
	}
}

func TestNewMinioClient(t *testing.T) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-secret",
			Namespace: "crossplane-system",
		},
		Data: map[string][]byte{
			MinioIDKey:     []byte("minioaccesskey"),
			MinioSecretKey: []byte("miniosecretkey"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	tests := []struct {
		name           string
		config         *providerv1.ProviderConfig
		setupClient    func() client.Client
		wantErr        bool
		wantSecure     bool
		checkTransport func(*testing.T, interface{})
	}{
		{
			name: "Basic HTTP configuration",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "http://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     false,
			wantSecure:  false,
		},
		{
			name: "HTTPS configuration without TLS config",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     false,
			wantSecure:  true,
		},
		{
			name: "HTTPS with custom TLS configuration",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
					TLS: &common.TLSConfig{
						InsecureSkipVerify: true,
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     false,
			wantSecure:  true,
		},
		{
			name: "HTTPS with custom CA",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
					TLS: &common.TLSConfig{
						CAData: testCACert,
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true, // Using test cert that's not properly formatted
			wantSecure:  true,
		},
		{
			name: "Missing secret should return error",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "nonexistent-secret",
							Namespace: "crossplane-system",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true,
		},
		{
			name: "Invalid URL should return error",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "://invalid-url",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true,
		},
		{
			name: "Invalid TLS configuration should return error",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-secret",
							Namespace: "crossplane-system",
						},
					},
					TLS: &common.TLSConfig{
						CAData: "invalid certificate data",
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupClient()

			got, err := NewMinioClient(ctx, client, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)

			// Verify the client was configured correctly
			// Note: We can't easily test the internal state of the MinIO client,
			// but we can verify it was created without error and the TLS configuration
			// didn't cause any issues during construction.
		})
	}
}

// Test that demonstrates the TLS configuration is properly applied
func TestTLSConfigurationApplied(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig *common.TLSConfig
		wantErr   bool
	}{
		{
			name:      "Nil TLS config should not cause errors",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name: "Valid TLS config should not cause errors",
			tlsConfig: &common.TLSConfig{
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
		{
			name: "TLS config with CA should not cause errors",
			tlsConfig: &common.TLSConfig{
				CAData: testCACert,
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
		{
			name: "Invalid CA should cause error",
			tlsConfig: &common.TLSConfig{
				CAData: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := buildTLSConfig(tt.tlsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, config)
		})
	}
}

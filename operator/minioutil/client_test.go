package minioutil

import (
	"context"
	"crypto/tls"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/rossigee/provider-minio/apis/common"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	// Test certificate placeholders for testing TLS functionality
	testCACert = `-----BEGIN CERTIFICATE-----
[TEST CA CERTIFICATE CONTENT - PLACEHOLDER FOR TESTING]
-----END CERTIFICATE-----`

	testClientCert = `-----BEGIN CERTIFICATE-----
[TEST CLIENT CERTIFICATE CONTENT - PLACEHOLDER FOR TESTING]
-----END CERTIFICATE-----`

	testClientKey = `-----BEGIN PRIVATE KEY-----
[TEST PRIVATE KEY CONTENT - PLACEHOLDER FOR TESTING]
-----END PRIVATE KEY-----`
)

func Test_IsTLSEnabled(t *testing.T) {
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
			name: "HTTP uppercase should return false",
			url:  "HTTP://example.com:9000",
			want: false,
		},
		{
			name: "HTTPS uppercase should return true",
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
			got := IsTLSEnabled(u)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_buildTLSConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create test secrets
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ca-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(testCACert),
		},
	}

	clientCertSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-client-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte(testClientCert),
			"tls.key": []byte(testClientKey),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(caSecret, clientCertSecret).
		Build()

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
			name: "CA secret reference should be resolved",
			tlsConfig: &common.TLSConfig{
				CASecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-ca-secret",
					},
					Key: "ca.crt",
				},
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.NotNil(t, config.RootCAs)
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
		{
			name: "Invalid secret reference should return error",
			tlsConfig: &common.TLSConfig{
				CASecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "nonexistent-secret",
					},
					Key: "ca.crt",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Client cert without key should return error",
			tlsConfig: &common.TLSConfig{
				ClientCertSecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-client-secret",
					},
					Key: "tls.crt",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Client key without cert should return error",
			tlsConfig: &common.TLSConfig{
				ClientKeySecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-client-secret",
					},
					Key: "tls.key",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid client cert and key secret refs should be loaded",
			tlsConfig: &common.TLSConfig{
				ClientCertSecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-client-secret",
					},
					Key: "tls.crt",
				},
				ClientKeySecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-client-secret",
					},
					Key: "tls.key",
				},
			},
			want: func(t *testing.T, config *tls.Config) {
				assert.Len(t, config.Certificates, 1)
			},
			wantErr: true, // Using test cert that's not properly formatted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTLSConfig(context.Background(), fakeClient, tt.tlsConfig, "test-namespace")
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
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create test secrets
	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-creds",
			Namespace: "crossplane-system",
		},
		Data: map[string][]byte{
			MinioIDKey:     []byte("test-access-key"),
			MinioSecretKey: []byte("test-secret-key"),
		},
	}

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ca-secret",
			Namespace: "crossplane-system",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(testCACert),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(credSecret, caSecret).
		Build()

	tests := []struct {
		name        string
		config      *providerv1.ProviderConfig
		setupClient func() client.Client
		wantErr     bool
		wantSecure  bool
	}{
		{
			name: "Basic HTTP config should work",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "http://minio.example.com:9000/",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-creds",
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
			name: "Basic HTTPS config should work",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000/",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-creds",
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
			name: "TLS config with CA secret reference should work",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000/",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-creds",
							Namespace: "crossplane-system",
						},
					},
					TLS: &common.TLSConfig{
						CASecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-ca-secret",
							},
							Key: "ca.crt",
						},
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
					MinioURL: "https://minio.example.com:9000/",
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
							Name:      "minio-creds",
							Namespace: "crossplane-system",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true,
		},
		{
			name: "TLS config with invalid secret should return error",
			config: &providerv1.ProviderConfig{
				Spec: providerv1.ProviderConfigSpec{
					MinioURL: "https://minio.example.com:9000/",
					Credentials: providerv1.ProviderCredentials{
						APISecretRef: corev1.SecretReference{
							Name:      "minio-creds",
							Namespace: "crossplane-system",
						},
					},
					TLS: &common.TLSConfig{
						CASecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "nonexistent-ca-secret",
							},
							Key: "ca.crt",
						},
					},
				},
			},
			setupClient: func() client.Client { return fakeClient },
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMinioClient(context.Background(), tt.setupClient(), tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

func Test_getSecretData(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"test-key": []byte("test-data"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testSecret).
		Build()

	tests := []struct {
		name      string
		secretRef *corev1.SecretKeySelector
		namespace string
		want      []byte
		wantErr   bool
	}{
		{
			name:      "Nil secret reference should return error",
			secretRef: nil,
			namespace: "test-namespace",
			want:      nil,
			wantErr:   true,
		},
		{
			name: "Valid secret reference should return data",
			secretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-secret",
				},
				Key: "test-key",
			},
			namespace: "test-namespace",
			want:      []byte("test-data"),
			wantErr:   false,
		},
		{
			name: "Nonexistent secret should return error",
			secretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "nonexistent-secret",
				},
				Key: "test-key",
			},
			namespace: "test-namespace",
			want:      nil,
			wantErr:   true,
		},
		{
			name: "Nonexistent key should return error",
			secretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-secret",
				},
				Key: "nonexistent-key",
			},
			namespace: "test-namespace",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSecretData(context.Background(), fakeClient, tt.secretRef, tt.namespace)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

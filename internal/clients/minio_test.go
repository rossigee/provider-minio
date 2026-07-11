package clients

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConfigWithAPISecretRef(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"accessKey": []byte("testadmin"),
			"secretKey": []byte("testsecret123"),
		},
	}
	secret.Name = "test-secret"
	secret.Namespace = "default"

	pc := &v1.ProviderConfig{
		Spec: v1.ProviderConfigSpec{
			MinioURL: "https://minio.example.com:9000",
			Credentials: v1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				APISecretRef: corev1.SecretReference{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithObjects(secret).
		Build()

	cfg, err := GetConfig(context.Background(), client, pc)
	require.NoError(t, err)
	require.Equal(t, "https://minio.example.com:9000", cfg.Endpoint)
	require.Equal(t, "testadmin", cfg.AccessKey)
	require.Equal(t, "testsecret123", cfg.SecretKey)
	require.True(t, cfg.UseSSL)
}

func TestGetConfigWithSecretRef(t *testing.T) {
	jsonData := []byte(`{"AccessKey":"testadmin","SecretKey":"testsecret123"}`)
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"credentials": jsonData,
		},
	}
	secret.Name = "test-secret"
	secret.Namespace = "default"

	pc := &v1.ProviderConfig{
		Spec: v1.ProviderConfigSpec{
			MinioURL: "http://minio.example.com:9000",
			Credentials: v1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "test-secret",
							Namespace: "default",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithObjects(secret).
		Build()

	cfg, err := GetConfig(context.Background(), client, pc)
	require.NoError(t, err)
	require.Equal(t, "http://minio.example.com:9000", cfg.Endpoint)
	require.Equal(t, "testadmin", cfg.AccessKey)
	require.Equal(t, "testsecret123", cfg.SecretKey)
	require.False(t, cfg.UseSSL)
}

func TestGetConfigNoSecretRef(t *testing.T) {
	pc := &v1.ProviderConfig{
		Spec: v1.ProviderConfigSpec{
			Credentials: v1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
			},
		},
	}

	client := fake.NewClientBuilder().Build()

	_, err := GetConfig(context.Background(), client, pc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no secret reference provided")
}

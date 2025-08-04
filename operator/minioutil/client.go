package minioutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rossigee/provider-minio/apis/common"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MinioIDKey     = "AWS_ACCESS_KEY_ID"
	MinioSecretKey = "AWS_SECRET_ACCESS_KEY"
)

// NewMinioClient returns a new minio client according to the given provider config.
func NewMinioClient(ctx context.Context, c client.Client, config *providerv1.ProviderConfig) (*minio.Client, error) {
	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: config.Spec.Credentials.APISecretRef.Name, Namespace: config.Spec.Credentials.APISecretRef.Namespace}
	err := c.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Spec.MinioURL)
	if err != nil {
		return nil, err
	}

	options := &minio.Options{
		Creds:  credentials.NewStaticV4(string(secret.Data[MinioIDKey]), string(secret.Data[MinioSecretKey]), ""),
		Secure: IsTLSEnabled(parsed),
	}

	// Apply custom TLS configuration if provided
	if config.Spec.TLS != nil {
		// Use the same namespace as the credentials for TLS secrets
		tlsConfig, err := buildTLSConfig(ctx, c, config.Spec.TLS, config.Spec.Credentials.APISecretRef.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS configuration: %w", err)
		}

		// Create a custom transport with the TLS config
		transport := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		options.Transport = transport
	}

	return minio.New(parsed.Host, options)
}

// IsTLSEnabled returns false if the scheme is explicitly set to `http` or `HTTP`
func IsTLSEnabled(u *url.URL) bool {
	return !strings.EqualFold(u.Scheme, "http")
}

// buildTLSConfig creates a tls.Config based on the provided common.TLSConfig
func buildTLSConfig(ctx context.Context, c client.Client, tlsConfig *common.TLSConfig, namespace string) (*tls.Config, error) {
	if tlsConfig == nil {
		return &tls.Config{}, nil
	}

	config := &tls.Config{
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}

	// Handle CA certificate from secret reference
	if tlsConfig.CASecretRef != nil {
		caData, err := getSecretData(ctx, c, tlsConfig.CASecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get CA certificate from secret: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to parse CA certificate from secret")
		}
		config.RootCAs = caCertPool
	}

	// Handle client certificate and key for mutual TLS from secret references
	if tlsConfig.ClientCertSecretRef != nil && tlsConfig.ClientKeySecretRef != nil {
		certData, err := getSecretData(ctx, c, tlsConfig.ClientCertSecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get client certificate from secret: %w", err)
		}

		keyData, err := getSecretData(ctx, c, tlsConfig.ClientKeySecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get client key from secret: %w", err)
		}

		cert, err := tls.X509KeyPair(certData, keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	} else if tlsConfig.ClientCertSecretRef != nil || tlsConfig.ClientKeySecretRef != nil {
		return nil, fmt.Errorf("both client certificate and key secret references must be provided for mutual TLS")
	}

	return config, nil
}

// getSecretData retrieves data from a Kubernetes secret using the provided secret key selector
func getSecretData(ctx context.Context, c client.Client, secretRef *corev1.SecretKeySelector, namespace string) ([]byte, error) {
	if secretRef == nil {
		return nil, fmt.Errorf("secret reference is nil")
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: namespace,
	}

	err := c.Get(ctx, key, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s in namespace %s: %w", secretRef.Name, namespace, err)
	}

	data, exists := secret.Data[secretRef.Key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in secret %s", secretRef.Key, secretRef.Name)
	}

	return data, nil
}

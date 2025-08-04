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
		Secure: isTLSEnabled(parsed),
	}

	// Apply custom TLS configuration if provided
	if config.Spec.TLS != nil {
		tlsConfig, err := buildTLSConfig(config.Spec.TLS)
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

// isTLSEnabled returns false if the scheme is explicitly set to `http` or `HTTP`
func isTLSEnabled(u *url.URL) bool {
	return !strings.EqualFold(u.Scheme, "http")
}

// buildTLSConfig creates a tls.Config based on the provided common.TLSConfig
func buildTLSConfig(tlsConfig *common.TLSConfig) (*tls.Config, error) {
	if tlsConfig == nil {
		return &tls.Config{}, nil
	}

	config := &tls.Config{
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}

	// Handle CA certificate
	if tlsConfig.CAData != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(tlsConfig.CAData)) {
			return nil, fmt.Errorf("failed to parse CA certificate data")
		}
		config.RootCAs = caCertPool
	}

	// Handle client certificate and key for mutual TLS
	if tlsConfig.ClientCertData != "" && tlsConfig.ClientKeyData != "" {
		cert, err := tls.X509KeyPair([]byte(tlsConfig.ClientCertData), []byte(tlsConfig.ClientKeyData))
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	} else if tlsConfig.ClientCertData != "" || tlsConfig.ClientKeyData != "" {
		return nil, fmt.Errorf("both client certificate and key must be provided for mutual TLS")
	}

	return config, nil
}

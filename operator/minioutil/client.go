package minioutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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
	var key client.ObjectKey
	var secretKey string

	// Use APISecretRef if available, otherwise fallback to SecretRef
	if config.Spec.Credentials.APISecretRef.Name != "" {
		key = client.ObjectKey{Name: config.Spec.Credentials.APISecretRef.Name, Namespace: config.Spec.Credentials.APISecretRef.Namespace}
		secretKey = "" // APISecretRef uses MinioIDKey and MinioSecretKey
	} else if config.Spec.Credentials.SecretRef != nil && config.Spec.Credentials.SecretRef.Name != "" {
		key = client.ObjectKey{Name: config.Spec.Credentials.SecretRef.Name, Namespace: config.Spec.Credentials.SecretRef.Namespace}
		secretKey = config.Spec.Credentials.SecretRef.Key
	} else {
		return nil, fmt.Errorf("no valid credentials reference found: APISecretRef or SecretRef must be provided with non-empty name")
	}

	err := c.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Spec.MinioURL)
	if err != nil {
		return nil, err
	}

	var accessKey, secretKeyValue string
	if secretKey != "" {
		// Using SecretRef with JSON data
		data, exists := secret.Data[secretKey]
		if !exists {
			return nil, fmt.Errorf("secret key %q not found in secret %s", secretKey, key)
		}
		var creds map[string]string
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal secret data: %w", err)
		}
		accessKey = creds["accessKey"]
		secretKeyValue = creds["secretKey"]
	} else {
		// Using APISecretRef with direct keys
		accessKey = string(secret.Data[MinioIDKey])
		secretKeyValue = string(secret.Data[MinioSecretKey])
	}

	options := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKeyValue, ""),
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
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify, // #nosec G402 -- gated by common.TLSConfig from ProviderConfig
	}

	// Handle CA certificate
	caData, err := getTLSData(ctx, c, namespace, tlsConfig.CAData, tlsConfig.CASecretRef, tlsConfig.CAConfigMapRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate data: %w", err)
	}
	if caData != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caData)) {
			return nil, fmt.Errorf("failed to parse CA certificate data")
		}
		config.RootCAs = caCertPool
	}

	// Handle client certificate
	clientCertData, err := getTLSData(ctx, c, namespace, tlsConfig.ClientCertData, tlsConfig.ClientCertSecretRef, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get client certificate data: %w", err)
	}

	// Handle client key
	clientKeyData, err := getTLSData(ctx, c, namespace, tlsConfig.ClientKeyData, tlsConfig.ClientKeySecretRef, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get client key data: %w", err)
	}

	// Handle client certificate and key for mutual TLS
	if clientCertData != "" && clientKeyData != "" {
		cert, err := tls.X509KeyPair([]byte(clientCertData), []byte(clientKeyData))
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	} else if clientCertData != "" || clientKeyData != "" {
		return nil, fmt.Errorf("both client certificate and key must be provided for mutual TLS")
	}

	return config, nil
}

// getTLSData retrieves TLS data from inline data, secret reference, or configmap reference
func getTLSData(ctx context.Context, c client.Client, namespace string, inlineData string, secretRef *corev1.SecretKeySelector, configMapRef *corev1.ConfigMapKeySelector) (string, error) {
	// Priority: inline data > secret ref > configmap ref
	if inlineData != "" {
		return inlineData, nil
	}

	if secretRef != nil {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Name: secretRef.Name, Namespace: namespace}
		if err := c.Get(ctx, key, secret); err != nil {
			return "", fmt.Errorf("failed to get secret %s: %w", secretRef.Name, err)
		}
		data, ok := secret.Data[secretRef.Key]
		if !ok {
			return "", fmt.Errorf("key %s not found in secret %s", secretRef.Key, secretRef.Name)
		}
		return string(data), nil
	}

	if configMapRef != nil {
		configMap := &corev1.ConfigMap{}
		key := client.ObjectKey{Name: configMapRef.Name, Namespace: namespace}
		if err := c.Get(ctx, key, configMap); err != nil {
			return "", fmt.Errorf("failed to get configmap %s: %w", configMapRef.Name, err)
		}
		data, ok := configMap.Data[configMapRef.Key]
		if !ok {
			return "", fmt.Errorf("key %s not found in configmap %s", configMapRef.Key, configMapRef.Name)
		}
		return data, nil
	}

	return "", nil
}

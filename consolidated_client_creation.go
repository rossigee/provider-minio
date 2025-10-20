// consolidated_client_creation.go
// Example of how to consolidate duplicate client creation logic

package clients

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConsolidatedConfig represents the complete MinIO client configuration
type ConsolidatedConfig struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	Region          string
	UseSSL          bool
	TLSCertData     []byte
	TLSKeyData      []byte
	CACertData      []byte
	InsecureSkipVerify bool
}

// NewConsolidatedMinioClient creates a MinIO client using consolidated logic
// This replaces both minioutil.NewMinioClient and clients.GetConfig
func NewConsolidatedMinioClient(ctx context.Context, config *ConsolidatedConfig) (*minio.Client, error) {
	// Set up TLS configuration if certificates are provided
	var transport *http.Transport
	if config.TLSCertData != nil && config.TLSKeyData != nil {
		cert, err := tls.X509KeyPair(config.TLSCertData, config.TLSKeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificates: %w", err)
		}

		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: config.InsecureSkipVerify,
			},
		}

		if config.CACertData != nil {
			caCertPool := tls.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(config.CACertData); !ok {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			transport.TLSClientConfig.RootCAs = caCertPool
		}
	}

	// Create MinIO client options
	opts := &minio.Options{
		Creds: credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
		Transport: transport,
	}

	// Create and return the client
	return minio.New(config.Endpoint, opts)
}

// GetConfigFromAPISecretRef extracts configuration from a Kubernetes secret
// This consolidates the APISecretRef handling logic used in both functions
func GetConfigFromAPISecretRef(ctx context.Context, secretRef *SecretReference, c client.Client) (*ConsolidatedConfig, error) {
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: secretRef.Namespace,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	config := &ConsolidatedConfig{
		AccessKey: string(secret.Data["accessKey"]),
		SecretKey: string(secret.Data["secretKey"]),
		Endpoint:  string(secret.Data["endpoint"]),
		Region:    string(secret.Data["region"]),
	}

	// Handle optional TLS data
	if caBundle, ok := secret.Data["caBundle"]; ok {
		config.CACertData = caBundle
	}

	if clientCert, ok := secret.Data["clientCert"]; ok {
		config.TLSCertData = clientCert
	}

	if clientKey, ok := secret.Data["clientKey"]; ok {
		config.TLSKeyData = clientKey
	}

	// Set defaults
	if config.Region == "" {
		config.Region = "us-east-1"
	}

	config.UseSSL = true // Default to SSL

	return config, nil
}

// Example usage in XRD composition pipeline:
// func (r *BucketClaimReconciler) reconcileXRDComposition(ctx context.Context, bc *v1beta1.BucketClaim) error {
//     config, err := GetConfigFromAPISecretRef(ctx, bc.Spec.CredentialsSecretRef, r.Client)
//     if err != nil {
//         return err
//     }
//
//     client, err := NewConsolidatedMinioClient(ctx, config)
//     if err != nil {
//         return err
//     }
//
//     // Use client for MinIO operations
//     return nil
// }
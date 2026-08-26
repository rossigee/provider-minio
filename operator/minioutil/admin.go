package minioutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
	providerv1 "github.com/rossigee/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewMinioAdmin returns a new minio admin client that can manage users and IAM.
// It can be used to assign a policy to a user.
func NewMinioAdmin(ctx context.Context, c client.Client, config *providerv1.ProviderConfig) (*madmin.AdminClient, error) {
	secret := &corev1.Secret{}
	var key client.ObjectKey
	var tlsNamespace string

	// Use APISecretRef if available, otherwise fallback to SecretRef
	if config.Spec.Credentials.APISecretRef.Name != "" {
		key = client.ObjectKey{Name: config.Spec.Credentials.APISecretRef.Name, Namespace: config.Spec.Credentials.APISecretRef.Namespace}
		tlsNamespace = config.Spec.Credentials.APISecretRef.Namespace
	} else if config.Spec.Credentials.SecretRef != nil && config.Spec.Credentials.SecretRef.Name != "" {
		key = client.ObjectKey{Name: config.Spec.Credentials.SecretRef.Name, Namespace: config.Spec.Credentials.SecretRef.Namespace}
		tlsNamespace = config.Spec.Credentials.SecretRef.Namespace
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

	// Determine which format we're using and extract credentials
	var secretKey string
	var accessKey, secretKeyValue string

	// Use APISecretRef if available, otherwise fallback to SecretRef
	if config.Spec.Credentials.APISecretRef.Name != "" {
		secretKey = "" // APISecretRef uses MinioIDKey and MinioSecretKey
	} else if config.Spec.Credentials.SecretRef != nil && config.Spec.Credentials.SecretRef.Name != "" {
		secretKey = config.Spec.Credentials.SecretRef.Key
	}

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

	adminClient, err := madmin.NewWithOptions(parsed.Host, &madmin.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKeyValue, ""),
		Secure: IsTLSEnabled(parsed),
	})
	if err != nil {
		return nil, err
	}

	// Apply custom TLS configuration if provided
	if config.Spec.TLS != nil {
		tlsConfig, err := buildTLSConfig(ctx, c, config.Spec.TLS, tlsNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS configuration: %w", err)
		}

		// Create a custom transport with the TLS config
		transport := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		adminClient.SetCustomTransport(transport)
	}

	return adminClient, nil
}

package minioutil

import (
	"context"
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
	key := client.ObjectKey{Name: config.Spec.Credentials.APISecretRef.Name, Namespace: config.Spec.Credentials.APISecretRef.Namespace}
	err := c.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Spec.MinioURL)
	if err != nil {
		return nil, err
	}

	adminClient, err := madmin.NewWithOptions(parsed.Host, &madmin.Options{
		Creds:  credentials.NewStaticV4(string(secret.Data[MinioIDKey]), string(secret.Data[MinioSecretKey]), ""),
		Secure: IsTLSEnabled(parsed),
	})
	if err != nil {
		return nil, err
	}

	// Apply custom TLS configuration if provided
	if config.Spec.TLS != nil {
		tlsConfig, err := buildTLSConfig(ctx, c, config.Spec.TLS, config.Spec.Credentials.APISecretRef.Namespace)
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

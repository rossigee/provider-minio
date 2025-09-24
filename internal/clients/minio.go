package clients

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/minio/madmin-go/v3"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/rossigee/provider-minio/apis/provider/v1"
)

const (
	errGetProviderConfig = "cannot get provider config"
	errGetConnectionSecret = "cannot get connection secret"
	errUnmarshalCredentials = "cannot unmarshal credentials"
	errFmtUnsupportedCredSource = "credentials source %q is not currently supported"
)

// Config contains configuration for the MinIO client
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// GetConfig extracts the MinIO configuration from a ProviderConfig
func GetConfig(ctx context.Context, c client.Client, pc *v1.ProviderConfig) (*Config, error) {
	switch {
	case pc.Spec.Credentials.Source == xpv1.CredentialsSourceSecret:
		secretRef := &xpv1.SecretReference{
			Name:      pc.Spec.Credentials.SecretRef.Name,
			Namespace: pc.Spec.Credentials.SecretRef.Namespace,
		}
		return getConfigFromSecret(ctx, c, secretRef, pc.Spec.Credentials.SecretRef.Key)
	default:
		return nil, errors.Errorf(errFmtUnsupportedCredSource, pc.Spec.Credentials.Source)
	}
}

func getConfigFromSecret(ctx context.Context, c client.Client, ref *xpv1.SecretReference, key string) (*Config, error) {
	if ref == nil {
		return nil, errors.New("no secret reference provided")
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := c.Get(ctx, nn, secret); err != nil {
		return nil, errors.Wrap(err, errGetConnectionSecret)
	}

	var cfg Config
	if err := json.Unmarshal(secret.Data[key], &cfg); err != nil {
		return nil, errors.Wrap(err, errUnmarshalCredentials)
	}

	return &cfg, nil
}

// NewMinIOClient creates a new MinIO admin client
func NewMinIOClient(cfg Config) (*madmin.AdminClient, error) {
	client, err := madmin.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	return client, nil
}

package clients

import (
	"context"
	"encoding/json"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	v1 "github.com/rossigee/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errGetProviderConfig        = "cannot get provider config"
	errGetConnectionSecret      = "cannot get connection secret"
	errUnmarshalCredentials     = "cannot unmarshal credentials"
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
	useSSL := pc.Spec.MinioURL != "" && pc.Spec.MinioURL[:8] == "https://"

	switch pc.Spec.Credentials.Source {
	case xpv1.CredentialsSourceSecret:
		// Support both apiSecretRef (standard Crossplane pattern) and secretRef (JSON key pattern)
		if pc.Spec.Credentials.APISecretRef.Name != "" {
			ref := &xpv1.SecretReference{
				Name:      pc.Spec.Credentials.APISecretRef.Name,
				Namespace: pc.Spec.Credentials.APISecretRef.Namespace,
			}
			return getConfigFromAPISecretRef(ctx, c, ref, pc.Spec.MinioURL, useSSL)
		} else if pc.Spec.Credentials.SecretRef != nil {
			ref := &xpv1.SecretReference{
				Name:      pc.Spec.Credentials.SecretRef.Name,
				Namespace: pc.Spec.Credentials.SecretRef.Namespace,
			}
			return getConfigFromSecret(ctx, c, ref, pc.Spec.Credentials.SecretRef.Key, pc.Spec.MinioURL, useSSL)
		}
		return nil, errors.New("no secret reference provided")
	default:
		return nil, errors.Errorf(errFmtUnsupportedCredSource, pc.Spec.Credentials.Source)
	}
}

func getConfigFromAPISecretRef(ctx context.Context, c client.Client, ref *xpv1.SecretReference, endpoint string, useSSL bool) (*Config, error) {
	if ref == nil {
		return nil, errors.New("no secret reference provided")
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := c.Get(ctx, nn, secret); err != nil {
		return nil, errors.Wrap(err, errGetConnectionSecret)
	}

	cfg := &Config{
		Endpoint:  endpoint,
		AccessKey: string(secret.Data["accessKey"]),
		SecretKey: string(secret.Data["secretKey"]),
		UseSSL:    useSSL,
	}

	return cfg, nil
}

func getConfigFromSecret(ctx context.Context, c client.Client, ref *xpv1.SecretReference, key string, endpoint string, useSSL bool) (*Config, error) {
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

	cfg.Endpoint = endpoint
	cfg.UseSSL = useSSL

	return &cfg, nil
}

// NewMinIOClient creates a new MinIO admin client
func NewMinIOClient(cfg Config) (*madmin.AdminClient, error) {
	client, err := madmin.NewWithOptions(cfg.Endpoint, &madmin.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

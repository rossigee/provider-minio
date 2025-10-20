package clients

import (
	"context"
	"encoding/json"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/rossigee/provider-minio/apis/minio/v1beta1"
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

// BucketConfig contains configuration for MinIO operations from BucketClaim
type BucketConfig struct {
	Credentials *credentials.Credentials
	Endpoint    string
	Region      string
	UseSSL      bool
}

// GetConfig extracts the MinIO configuration from a ProviderConfig
func GetConfig(ctx context.Context, c client.Client, pc *v1.ProviderConfig) (*Config, error) {
	switch pc.Spec.Credentials.Source {
	case xpv1.CredentialsSourceSecret:
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

// GetBucketConfig extracts the MinIO configuration from a BucketClaim
// This supports XRD compositions that use APISecretRef directly
func GetBucketConfig(ctx context.Context, c client.Client, bc *v1beta1.BucketClaim) (*BucketConfig, error) {
	if bc.Spec.CredentialsSecretRef == nil {
		return nil, errors.New("no credentials secret reference provided in BucketClaim")
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: bc.Spec.CredentialsSecretRef.Namespace,
		Name:      bc.Spec.CredentialsSecretRef.Name,
	}
	if err := c.Get(ctx, nn, secret); err != nil {
		return nil, errors.Wrap(err, errGetConnectionSecret)
	}

	// Extract credentials from secret
	accessKey := string(secret.Data["accessKey"])
	secretKey := string(secret.Data["secretKey"])
	endpoint := string(secret.Data["endpoint"])

	if accessKey == "" || secretKey == "" {
		return nil, errors.New("accessKey and secretKey are required in credentials secret")
	}

	// Default values
	region := bc.Spec.Region
	if region == "" {
		region = "us-east-1"
	}

	useSSL := true // Default to SSL
	if endpoint == "" {
		return nil, errors.New("endpoint is required in credentials secret")
	}

	return &BucketConfig{
		Credentials: credentials.NewStaticV4(accessKey, secretKey, ""),
		Endpoint:    endpoint,
		Region:      region,
		UseSSL:      useSSL,
	}, nil
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

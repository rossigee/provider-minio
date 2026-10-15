package user

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotUser = fmt.Errorf("managed resource is not a user")
)

type connector struct {
	kube     client.Client
	recorder event.Recorder
	usage    resource.ModernTracker

	// newAdmin is a seam so tests can substitute a fake. When nil the real
	// MinIO admin client is used.
	newAdmin adminFactory
}

type adminFactory func(context.Context, client.Client, *providerv1beta1.ProviderConfig) (minioutil.UserAdmin, error)

func newRealAdmin(ctx context.Context, kube client.Client, config *providerv1beta1.ProviderConfig) (minioutil.UserAdmin, error) {
	return minioutil.NewMinioAdmin(ctx, kube, config)
}

type userClient struct {
	ma          minioutil.UserAdmin
	kube        client.Client
	recorder    event.Recorder
	url         *url.URL
	tlsSettings bool

	// newCredentialClient builds the S3 client used to verify that the
	// credentials just written actually work. It is a field so tests can
	// substitute a fake; when nil the real client is built from the connection
	// secret.
	newCredentialClient func(ctx context.Context, accessKey, secretKey string) (minioutil.UserS3, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("connecting resource")

	err := c.usage.Track(ctx, mg.(resource.ModernManaged))
	if err != nil {
		return nil, err
	}

	userv1beta1, ok := mg.(*miniov1beta1.User)
	if !ok {
		return nil, errNotUser
	}

	config, err := c.getProviderConfig(ctx, userv1beta1)
	if err != nil {
		return nil, err
	}

	maker := c.newAdmin
	if maker == nil {
		maker = newRealAdmin
	}

	ma, err := maker(ctx, c.kube, config)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Spec.MinioURL)
	if err != nil {
		return nil, err
	}

	uc := &userClient{
		ma:          ma,
		kube:        c.kube,
		recorder:    c.recorder,
		url:         parsed,
		tlsSettings: minioutil.IsTLSEnabled(parsed),
	}

	return uc, nil
}

func (c *connector) getProviderConfig(ctx context.Context, user *miniov1beta1.User) (*providerv1beta1.ProviderConfig, error) {
	configName := user.GetProviderConfigReference().Name
	config := &providerv1beta1.ProviderConfig{}
	err := c.kube.Get(ctx, client.ObjectKey{Name: configName}, config)
	return config, err
}

// credentialClient returns the S3 client used to confirm that credentials
// written for a user actually work. The real implementation builds a client
// from the connection secret; tests substitute their own.
func (u *userClient) credentialClient(accessKey, secretKey string) (minioutil.UserS3, error) {
	if u.newCredentialClient != nil {
		return u.newCredentialClient(context.Background(), accessKey, secretKey)
	}
	return minio.New(u.url.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: u.tlsSettings,
	})
}

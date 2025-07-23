package serviceaccount

import (
	"context"

	"github.com/minio/madmin-go/v4"
)

// MinioAdminClient is an interface that wraps the MinIO admin client methods we use
type MinioAdminClient interface {
	AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error)
	InfoServiceAccount(ctx context.Context, accessKey string) (madmin.InfoServiceAccountResp, error)
	UpdateServiceAccount(ctx context.Context, accessKey string, opts madmin.UpdateServiceAccountReq) error
	DeleteServiceAccount(ctx context.Context, accessKey string) error
}

// Ensure madmin.AdminClient implements our interface
var _ MinioAdminClient = (*madmin.AdminClient)(nil)

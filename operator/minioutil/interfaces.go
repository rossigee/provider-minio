package minioutil

import (
	"context"
	"encoding/json"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/tags"
)

// The interfaces below narrow the MinIO client surface to exactly what each
// controller uses. The concrete *madmin.AdminClient and *minio.Client satisfy
// them structurally, so NewMinioAdmin and NewMinioClient continue to satisfy
// every controller unchanged; the point is that a test can supply a fake.
//
// They are deliberately split per resource rather than exposing one large
// "MinIO" interface. A single interface would force a fake to implement all 27
// methods to test any one controller, and would let a controller call methods
// it has no business using.

// PolicyAdmin is the MinIO admin API surface used by the Policy controller.
type PolicyAdmin interface {
	ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error)
	AddCannedPolicy(ctx context.Context, policyName string, policy []byte) error
	RemoveCannedPolicy(ctx context.Context, policyName string) error
}

// UserAdmin is the MinIO admin API surface used by the User controller.
type UserAdmin interface {
	AddUser(ctx context.Context, accessKey, secretKey string) error
	RemoveUser(ctx context.Context, accessKey string) error
	ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error)
	GetUserInfo(ctx context.Context, name string) (madmin.UserInfo, error)
	SetUser(ctx context.Context, accessKey, secretKey string, status madmin.AccountStatus) error
	AttachPolicy(ctx context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error)
	DetachPolicy(ctx context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error)
}

// UserS3 is the additional S3 API surface used by the User controller to
// validate the credentials it just wrote.
type UserS3 interface {
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
}

// ServiceAccountAdmin is the MinIO admin API surface used by the
// ServiceAccount controller.
type ServiceAccountAdmin interface {
	GetUserInfo(ctx context.Context, name string) (madmin.UserInfo, error)
	AttachPolicy(ctx context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error)
	DetachPolicy(ctx context.Context, r madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error)
	AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error)
	InfoServiceAccount(ctx context.Context, accessKey string) (madmin.InfoServiceAccountResp, error)
	UpdateServiceAccount(ctx context.Context, accessKey string, opts madmin.UpdateServiceAccountReq) error
	DeleteServiceAccount(ctx context.Context, serviceAccount string) error
}

// BucketS3 is the MinIO S3 API surface used by the Bucket controller.
type BucketS3 interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	GetBucketPolicy(ctx context.Context, bucketName string) (string, error)
	GetBucketTagging(ctx context.Context, bucketName string) (*tags.Tags, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	SetBucketPolicy(ctx context.Context, bucketName, policy string) error
	SetBucketTagging(ctx context.Context, bucketName string, tags *tags.Tags) error
	RemoveBucketTagging(ctx context.Context, bucketName string) error
	RemoveBucket(ctx context.Context, bucketName string) error
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
	GetObjectLockConfig(ctx context.Context, bucketName string) (objectLock string, mode *minio.RetentionMode, validity *uint, unit *minio.ValidityUnit, err error)
}

// NotificationS3 is the MinIO S3 API surface used by the
// NotificationConfiguration controller.
type NotificationS3 interface {
	GetBucketNotification(ctx context.Context, bucketName string) (notification.Configuration, error)
	SetBucketNotification(ctx context.Context, bucketName string, config notification.Configuration) error
}

// Compile-time proof that the real clients still satisfy the narrowed
// interfaces. If a MinIO dependency upgrade changes a signature, this fails at
// compile time rather than silently breaking a fake in a test.
var (
	_ PolicyAdmin         = (*madmin.AdminClient)(nil)
	_ UserAdmin           = (*madmin.AdminClient)(nil)
	_ UserS3              = (*minio.Client)(nil)
	_ ServiceAccountAdmin = (*madmin.AdminClient)(nil)
	_ BucketS3            = (*minio.Client)(nil)
	_ NotificationS3      = (*minio.Client)(nil)
)

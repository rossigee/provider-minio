package serviceaccount

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
)

func (s *serviceAccountClient) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for MinIO service accounts
	// The admin client will be garbage collected
	return nil
}

var _ managed.ExternalClient = &serviceAccountClient{}
var _ managed.ExternalConnecter = &connector{}

package serviceaccount

import (
	"context"
)

func (c *serviceAccountClient) Disconnect(ctx context.Context) error {
	// No persistent connections to clean up for MinIO admin client
	return nil
}

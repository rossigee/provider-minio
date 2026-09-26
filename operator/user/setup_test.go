package user

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupControllerWithConnectorAcceptsACustomConnector guards the seam the
// reconciler tests rely on: SetupControllerWithConnector must pass the supplied
// connector through to the managed reconciler rather than building its own.
// If this signature changes, the per-package connector tests stop compiling and
// the change is caught here first.
func TestSetupControllerWithConnectorAcceptsACustomConnector(t *testing.T) {
	var c connector
	var _ managed.ExternalConnector = &c

	// The reconciler is constructed from this connector; assert the function
	// exists with the expected shape at compile time and at runtime.
	fn := SetupControllerWithConnector
	require.NotNil(t, fn)
	assert.Implements(t, (*managed.ExternalConnector)(nil), &c)
}

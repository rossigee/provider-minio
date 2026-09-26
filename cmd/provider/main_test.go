package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMainEnablesWatchBookmarks verifies the actual source, rather than
// re-asserting a literal. The previous version of this test constructed
// ptr.To(true) and asserted it was true, so it passed even if main.go had
// stopped enabling watch bookmarks entirely.
//
// Watch bookmarks let a dropped or stalled watch resume efficiently instead of
// relisting, so a silent regression here is worth failing a build over.
func TestMainEnablesWatchBookmarks(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)

	re := regexp.MustCompile(`(?s)cache\.Options\{.*?DefaultEnableWatchBookmarks:\s*ptr\.To\(true\)`)
	assert.Regexp(t, re, string(src),
		"main.go must set DefaultEnableWatchBookmarks to ptr.To(true) in its cache.Options")
}

// TestMainConfiguresLeaderElection documents the leader election defaults the
// deployment relies on for HA.
func TestMainConfiguresLeaderElection(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)

	assert.Contains(t, string(src), "LeaderElection",
		"main.go must configure leader election so replicas do not reconcile concurrently")
}

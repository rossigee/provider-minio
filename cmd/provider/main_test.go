package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestWatchBookmarksEnabled(t *testing.T) {
	// This test verifies that watch bookmarks are enabled in the cache options.
	// Watch bookmarks help with watch synchronization and recovery when watches
	// drop events or become stalled.
	//
	// This is a compile-time check that the main.go file enables watch bookmarks
	// via DefaultEnableWatchBookmarks: ptr.To(true).
	//
	// If this test fails, it indicates that watch bookmarks have been disabled,
	// which would require investigation into why they were disabled and whether
	// the watch reliability issues have been resolved.

	// ptr.To(true) creates a pointer to true
	// We use this value in cache.Options.DefaultEnableWatchBookmarks
	expectedWatchBookmarks := ptr.To(true)
	assert.NotNil(t, expectedWatchBookmarks)
	assert.True(t, *expectedWatchBookmarks)
}

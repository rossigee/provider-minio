/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/minio/madmin-go/v3"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects non-MinIO ProviderConfig", func(t *testing.T) {
		e := &external{}

		var pc resource.ProviderConfig

		_, err := e.List(ctx, pc, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a MinIO ProviderConfig")
	})

	t.Run("empty result on non-empty page token", func(t *testing.T) {
		e := &external{}

		result, err := e.List(ctx, &providerv1beta1.ProviderConfig{}, "2")
		require.NoError(t, err)
		assert.Empty(t, result.ExternalNames)
		assert.Empty(t, result.NextPageToken)
	})

	t.Run("returns sorted usernames without token", func(t *testing.T) {
		e := &external{
			listUsersFn: func(ctx context.Context) (map[string]madmin.UserInfo, error) {
				return map[string]madmin.UserInfo{
					"zeta":  {},
					"":      {},
					"alpha": {},
					"mid":   {},
				}, nil
			},
		}

		result, err := e.List(ctx, &providerv1beta1.ProviderConfig{}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"alpha", "mid", "zeta"}, result.ExternalNames)
		assert.Empty(t, result.NextPageToken)
	})

	t.Run("returns listing errors", func(t *testing.T) {
		e := &external{
			listUsersFn: func(ctx context.Context) (map[string]madmin.UserInfo, error) {
				return nil, assert.AnError
			},
		}

		_, err := e.List(ctx, &providerv1beta1.ProviderConfig{}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list users")
	})

	t.Run("returns credential errors", func(t *testing.T) {
		e := &external{}

		_, err := e.List(ctx, &providerv1beta1.ProviderConfig{}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), errGetCreds)
	})
}

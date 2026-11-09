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

package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/rossigee/provider-minio/apis"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
)

const mrdName = "users.user.minio.m.crossplane.io"

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(s))
	require.NoError(t, v1alpha1.AddToScheme(s))

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.ExternalInventory{}).
		Build()
}

func providerConfig() *providerv1beta1.ProviderConfig {
	pc := &providerv1beta1.ProviderConfig{}
	pc.SetName("default")

	return pc
}

func name(context.Context) (string, error) { return mrdName, nil }

func getInventory(t *testing.T, kube client.Client) *v1alpha1.ExternalInventory {
	t.Helper()

	inv := &v1alpha1.ExternalInventory{}
	require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: mrdName}, inv))

	return inv
}

func TestPublishWritesSortedItems(t *testing.T) {
	kube := newClient(t, providerConfig())
	list := func(context.Context, *providerv1beta1.ProviderConfig) ([]string, error) {
		return []string{"bob", "alice"}, nil
	}

	p := NewPublisher(kube, "default", time.Hour, list, name, logging.NewNopLogger())
	require.NoError(t, p.Publish(context.Background()))

	inv := getInventory(t, kube)
	assert.Equal(t, "ManagedResourceDefinition", inv.Spec.MRDRef.Kind)
	assert.Equal(t, mrdName, inv.Spec.MRDRef.Name)
	require.NotNil(t, inv.Spec.ProviderConfigRef)
	assert.Equal(t, "default", inv.Spec.ProviderConfigRef.Name)

	require.Len(t, inv.Status.Scopes, 1)
	assert.Empty(t, inv.Status.Scopes[0].Scope)
	assert.Empty(t, inv.Status.Scopes[0].Error)
	assert.Equal(t, []v1alpha1.ExternalInventoryItem{
		{ExternalName: "alice"},
		{ExternalName: "bob"},
	}, inv.Status.Scopes[0].Items)
	assert.NotNil(t, inv.Status.LastListed)
}

func TestPublishReplacesPreviousListing(t *testing.T) {
	kube := newClient(t, providerConfig())
	first := func(context.Context, *providerv1beta1.ProviderConfig) ([]string, error) {
		return []string{"alice", "bob"}, nil
	}
	second := func(context.Context, *providerv1beta1.ProviderConfig) ([]string, error) {
		return []string{"carol"}, nil
	}

	ctx := context.Background()
	require.NoError(t, NewPublisher(kube, "default", time.Hour, first, name, logging.NewNopLogger()).Publish(ctx))
	require.NoError(t, NewPublisher(kube, "default", time.Hour, second, name, logging.NewNopLogger()).Publish(ctx))

	inv := getInventory(t, kube)
	require.Len(t, inv.Status.Scopes, 1)
	assert.Equal(t, []v1alpha1.ExternalInventoryItem{{ExternalName: "carol"}}, inv.Status.Scopes[0].Items)
}

func TestPublishRecordsListingError(t *testing.T) {
	kube := newClient(t, providerConfig())
	list := func(context.Context, *providerv1beta1.ProviderConfig) ([]string, error) {
		return nil, errors.New("admin API unreachable")
	}

	p := NewPublisher(kube, "default", time.Hour, list, name, logging.NewNopLogger())
	require.NoError(t, p.Publish(context.Background()))

	inv := getInventory(t, kube)
	require.Len(t, inv.Status.Scopes, 1)
	assert.Equal(t, "admin API unreachable", inv.Status.Scopes[0].Error)
	assert.Empty(t, inv.Status.Scopes[0].Items)
	assert.Nil(t, inv.Status.LastListed, "a failed listing must not advance LastListed")
}

func TestPublishMissingProviderConfig(t *testing.T) {
	kube := newClient(t)
	list := func(context.Context, *providerv1beta1.ProviderConfig) ([]string, error) {
		t.Fatal("lister must not run without a ProviderConfig")
		return nil, nil
	}

	p := NewPublisher(kube, "default", time.Hour, list, name, logging.NewNopLogger())
	require.Error(t, p.Publish(context.Background()))

	err := kube.Get(context.Background(), types.NamespacedName{Name: mrdName}, &v1alpha1.ExternalInventory{})
	require.Error(t, err, "no inventory should be written without a ProviderConfig")
}

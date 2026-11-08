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
	"sort"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rossigee/provider-minio/internal/clients"

	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
)

// ListUsernames returns the usernames of all users on the MinIO deployment that
// the ProviderConfig points at, sorted. It is the lister that publishes the user
// ExternalInventory.
func ListUsernames(ctx context.Context, kube client.Client, cfg *providerv1beta1.ProviderConfig) ([]string, error) {
	conf, err := clients.GetConfig(ctx, kube, cfg)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	admin, err := clients.NewMinIOClient(*conf)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	users, err := admin.ListUsers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list users")
	}

	names := make([]string, 0, len(users))
	for name := range users {
		if name != "" {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names, nil
}

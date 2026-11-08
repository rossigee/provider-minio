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

// Package inventory publishes the external resources a provider can list as
// ExternalInventory objects, one per ManagedResourceDefinition, so that Crossplane
// can discover resources that are not yet managed.
package inventory

import (
	"context"
	"sort"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
)

// Lister returns the external names of the resources of one kind that the
// ProviderConfig can see.
type Lister func(ctx context.Context, pc *providerv1beta1.ProviderConfig) ([]string, error)

// MRDName returns the name of the ManagedResourceDefinition for the kind.
type MRDName func(ctx context.Context) (string, error)

// A Publisher periodically writes the ExternalInventory of one kind. It implements
// controller-runtime's Runnable, so it runs under the manager.
type Publisher struct {
	kube           client.Client
	providerConfig string
	interval       time.Duration
	list           Lister
	mrdName        MRDName
	log            logging.Logger
}

// NewPublisher returns a Publisher that lists with the named ProviderConfig every
// interval.
func NewPublisher(kube client.Client, providerConfig string, interval time.Duration, list Lister, mrdName MRDName, log logging.Logger) *Publisher {
	return &Publisher{
		kube:           kube,
		providerConfig: providerConfig,
		interval:       interval,
		list:           list,
		mrdName:        mrdName,
		log:            log,
	}
}

// Start publishes an inventory now and then every interval until ctx is done.
// A failed publish is logged and retried at the next interval.
func (p *Publisher) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		if err := p.Publish(ctx); err != nil {
			p.log.Info("Cannot publish ExternalInventory", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Publish lists the kind with the ProviderConfig and writes the result to the
// ExternalInventory of the MRD. A failed listing is recorded in the inventory as
// the error of its scope rather than returned, so consumers see it.
func (p *Publisher) Publish(ctx context.Context) error {
	name, err := p.mrdName(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot resolve ManagedResourceDefinition name")
	}

	pc := &providerv1beta1.ProviderConfig{}
	if err := p.kube.Get(ctx, types.NamespacedName{Name: p.providerConfig}, pc); err != nil {
		return errors.Wrapf(err, "cannot get ProviderConfig %q", p.providerConfig)
	}

	now := metav1.Now()
	scope := v1alpha1.ExternalInventoryScopeStatus{}

	names, listErr := p.list(ctx, pc)
	if listErr != nil {
		scope.Error = listErr.Error()
	} else {
		sort.Strings(names)
		for _, n := range names {
			scope.Items = append(scope.Items, v1alpha1.ExternalInventoryItem{ExternalName: n})
		}
	}

	inv := &v1alpha1.ExternalInventory{}
	inv.SetName(name)

	spec := v1alpha1.ExternalInventorySpec{
		MRDRef: xpv2.TypedReference{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "ManagedResourceDefinition",
			Name:       name,
		},
		ProviderConfigRef: &xpv2.Reference{Name: p.providerConfig},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, p.kube, inv, func() error {
		inv.Spec = spec
		return nil
	}); err != nil {
		return errors.Wrapf(err, "cannot write ExternalInventory %q", name)
	}

	inv.Status.Scopes = []v1alpha1.ExternalInventoryScopeStatus{scope}
	if listErr == nil {
		inv.Status.LastListed = &now
	}

	if err := p.kube.Status().Update(ctx, inv); err != nil {
		return errors.Wrapf(err, "cannot update ExternalInventory %q status", name)
	}

	return nil
}

/*
Copyright 2022 The Crossplane Authors.

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

// Package connection provides utilities for working with connection details.
package connection

import (
	"context"
	"crypto/tls"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errConnectStore    = "cannot connect to secret store"
	errWriteStore      = "cannot write to secret store"
	errReadStore       = "cannot read from secret store"
	errDeleteFromStore = "cannot delete from secret store"
	errGetStoreConfig  = "cannot get store config"
	errSecretConflict  = "cannot establish control of existing connection secret"

	errFmtNotOwnedBy = "existing secret is not owned by UID %q"
)

// StoreBuilderFn is a function that builds and returns a Store with a given
// store config.
type StoreBuilderFn func(ctx context.Context, local client.Client, tcfg *tls.Config, cfg v1.SecretStoreConfig) (Store, error)

// A DetailsManagerOption configures a DetailsManager.
type DetailsManagerOption func(*DetailsManager)

// WithStoreBuilder configures the StoreBuilder to use.
func WithStoreBuilder(sb StoreBuilderFn) DetailsManagerOption {
	return func(m *DetailsManager) {
		m.storeBuilder = sb
	}
}

// WithTLSConfig configures the TLS config to use.
func WithTLSConfig(tcfg *tls.Config) DetailsManagerOption {
	return func(m *DetailsManager) {
		m.tcfg = tcfg
	}
}

// DetailsManager is a connection details manager that satisfies the required
// interfaces to work with connection details by managing interaction with
// different store implementations.
type DetailsManager struct {
	client       client.Client
	newConfig    func() StoreConfig
	storeBuilder StoreBuilderFn
	tcfg         *tls.Config
}

// NewDetailsManager returns a new connection DetailsManager.
func NewDetailsManager(c client.Client, of schema.GroupVersionKind, o ...DetailsManagerOption) *DetailsManager {
	nc := func() StoreConfig {
		return resource.MustCreateObject(of, c.Scheme()).(StoreConfig)
	}

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_ = nc()

	m := &DetailsManager{
		client:       c,
		newConfig:    nc,
		storeBuilder: RuntimeStoreBuilder,
	}

	for _, mo := range o {
		mo(m)
	}

	return m
}

// PublishConnection publishes the supplied ConnectionDetails to a secret on
// the configured connection Store.
func (m *DetailsManager) PublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, conn managed.ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return false, nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	changed, err := ss.WriteKeyValues(ctx, store.NewSecret(so, store.KeyValues(conn)), SecretToWriteMustBeOwnedBy(so))
	return changed, errors.Wrap(err, errWriteStore)
}

// UnpublishConnection deletes connection details secret to the configured
// connection Store.
func (m *DetailsManager) UnpublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, conn managed.ConnectionDetails) error {
	// This resource didn't expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	return errors.Wrap(ss.DeleteKeyValues(ctx, store.NewSecret(so, store.KeyValues(conn)), SecretToDeleteMustBeOwnedBy(so)), errDeleteFromStore)
}

// FetchConnection fetches connection details of a given ConnectionSecretOwner.
func (m *DetailsManager) FetchConnection(ctx context.Context, so resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	// This resource does not want to expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil, nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, errConnectStore)
	}

	s := &store.Secret{}
	return managed.ConnectionDetails(s.Data), errors.Wrap(ss.ReadKeyValues(ctx, store.ScopedName{Name: p.Name, Scope: so.GetNamespace()}, s), errReadStore)
}

// PropagateConnection propagate connection details from one resource to another.
func (m *DetailsManager) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	// Either from does not expose a connection secret, or to does not want one.
	if from.GetPublishConnectionDetailsTo() == nil || to.GetPublishConnectionDetailsTo() == nil {
		return false, nil
	}

	ssFrom, err := m.connectStore(ctx, from.GetPublishConnectionDetailsTo())
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	sFrom := &store.Secret{}
	if err = ssFrom.ReadKeyValues(ctx, store.ScopedName{
		Name:  from.GetPublishConnectionDetailsTo().Name,
		Scope: from.GetNamespace(),
	}, sFrom); err != nil {
		return false, errors.Wrap(err, errReadStore)
	}

	// Make sure 'from' is the controller of the connection secret it references
	// before we propagate it. This ensures a resource cannot use Crossplane to
	// circumvent RBAC by propagating a secret it does not own.
	if sFrom.GetOwner() != string(from.GetUID()) {
		return false, errors.New(errSecretConflict)
	}

	ssTo, err := m.connectStore(ctx, to.GetPublishConnectionDetailsTo())
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	changed, err := ssTo.WriteKeyValues(ctx, store.NewSecret(to, sFrom.Data), SecretToWriteMustBeOwnedBy(to))
	return changed, errors.Wrap(err, errWriteStore)
}

func (m *DetailsManager) connectStore(ctx context.Context, p *v1.PublishConnectionDetailsTo) (Store, error) {
	sc := m.newConfig()
	if err := m.client.Get(ctx, types.NamespacedName{Name: p.SecretStoreConfigRef.Name}, sc); err != nil {
		return nil, errors.Wrap(err, errGetStoreConfig)
	}

	return m.storeBuilder(ctx, m.client, m.tcfg, sc.GetStoreConfig())
}

// SecretToWriteMustBeOwnedBy requires that the current object is a
// connection secret that is owned by an object with the supplied UID.
func SecretToWriteMustBeOwnedBy(so metav1.Object) store.WriteOption {
	return func(_ context.Context, current, _ *store.Secret) error {
		return secretMustBeOwnedBy(so, current)
	}
}

// SecretToDeleteMustBeOwnedBy requires that the current secret is owned by
// an object with the supplied UID.
func SecretToDeleteMustBeOwnedBy(so metav1.Object) store.DeleteOption {
	return func(_ context.Context, secret *store.Secret) error {
		return secretMustBeOwnedBy(so, secret)
	}
}

func secretMustBeOwnedBy(so metav1.Object, secret *store.Secret) error {
	if secret.Metadata == nil || secret.Metadata.GetOwnerUID() != string(so.GetUID()) {
		return errors.Errorf(errFmtNotOwnedBy, string(so.GetUID()))
	}
	return nil
}

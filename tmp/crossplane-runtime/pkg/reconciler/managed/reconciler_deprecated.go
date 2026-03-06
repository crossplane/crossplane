/*
Copyright 2025 The Crossplane Authors.

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

package managed

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// ExternalConnecter an alias to ExternalConnector.
//
// Deprecated: use ExternalConnector.
type ExternalConnecter = ExternalConnector

// TypedExternalConnecter an alias to TypedExternalConnector.
//
// Deprecated: use TypedExternalConnector.
type TypedExternalConnecter[managed resource.Managed] interface {
	TypedExternalConnector[managed]
}

// An ExternalDisconnector disconnects from a provider.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting
// from the provider.
//
//nolint:iface // We know it is a redundant interface.
type ExternalDisconnector interface {
	// Disconnect from the provider and close the ExternalClient.
	Disconnect(ctx context.Context) error
}

// ExternalDisconnecter an alias to ExternalDisconnector.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting
// from the provider.
//
//nolint:iface // We know it is a redundant interface
type ExternalDisconnecter interface {
	ExternalDisconnector
}

// NopDisconnecter aliases NopDisconnector.
//
// Deprecated: Use NopDisconnector.
type NopDisconnecter = NopDisconnector

// TODO: these types of aliases are only allowed in Go 1.23 and above.
// type TypedNopDisconnecter[managed resource.Managed] = TypedNopDisconnector[managed]
// type TypedNopDisconnecter[managed resource.Managed] = TypedNopDisconnector[managed]
// type TypedExternalConnectDisconnecterFns[managed resource.Managed] =  TypedExternalConnectDisconnectorFns[managed]

// NewNopDisconnecter an alias to NewNopDisconnector.
//
// Deprecated: use NewNopDisconnector.
func NewNopDisconnecter(c ExternalConnector) ExternalConnectDisconnector {
	return NewNopDisconnector(c)
}

// ExternalDisconnecterFn aliases ExternalDisconnectorFn.
//
// Deprecated: use ExternalDisconnectorFn.
type ExternalDisconnecterFn = ExternalDisconnectorFn

// ExternalConnectDisconnecterFns aliases ExternalConnectDisconnectorFns.
//
// Deprecated: use ExternalConnectDisconnectorFns.
type ExternalConnectDisconnecterFns = ExternalConnectDisconnectorFns

// NopConnecter aliases NopConnector.
//
// Deprecated: use NopConnector.
type NopConnecter = NopConnector

// WithExternalConnecter aliases WithExternalConnector.
//
// Deprecated: use WithExternalConnector.
func WithExternalConnecter(c ExternalConnector) ReconcilerOption {
	return WithExternalConnector(c)
}

// WithExternalConnectDisconnector specifies how the Reconciler should connect and disconnect to the API
// used to sync and delete external resources.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting from the provider.
func WithExternalConnectDisconnector(c ExternalConnectDisconnector) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnector = c
	}
}

// WithExternalConnectDisconnecter aliases WithExternalConnectDisconnector.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting from the provider.
func WithExternalConnectDisconnecter(c ExternalConnectDisconnector) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnector = c
	}
}

// WithTypedExternalConnectDisconnector specifies how the Reconciler should connect and disconnect to the API
// used to sync and delete external resources.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting from the provider.
func WithTypedExternalConnectDisconnector[managed resource.Managed](c TypedExternalConnectDisconnector[managed]) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnector = &typedExternalConnectDisconnectorWrapper[managed]{c}
	}
}

// WithTypedExternalConnectDisconnecter aliases WithTypedExternalConnectDisconnector.
//
// Deprecated: Please use Disconnect() on the ExternalClient for disconnecting from the provider.
func WithTypedExternalConnectDisconnecter[managed resource.Managed](c TypedExternalConnectDisconnector[managed]) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnector = &typedExternalConnectDisconnectorWrapper[managed]{c}
	}
}

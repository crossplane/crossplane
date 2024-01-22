// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package composite

import (
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// A ResourceName uniquely identifies the composed resource within a Composition
// and within Composition Function gRPC calls. This is not the metadata.name of
// the actual composed resource instance; rather it is the name of an entry in a
// Composition's resources array, and/or a RunFunctionRequest's observed/desired
// resources object.
type ResourceName string

// A ComposedResource is an output of the composition process.
type ComposedResource struct {
	// ResourceName of the composed resource.
	ResourceName ResourceName

	// Ready indicates whether this composed resource is ready - i.e. whether
	// all of its readiness checks passed.
	Ready bool
}

// ComposedResourceState represents a composed resource (either desired or
// observed).
type ComposedResourceState struct {
	Resource          resource.Composed
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}

// ComposedResourceStates tracks the state of composed resources.
type ComposedResourceStates map[ResourceName]ComposedResourceState

// ComposedResourceTemplates are the P&T templates for composed resources.
type ComposedResourceTemplates map[ResourceName]v1.ComposedTemplate

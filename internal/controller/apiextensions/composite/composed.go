/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package composite

import (
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// A CompositeResource is an output of the composition process.
type CompositeResource struct { //nolint:revive // stick with CompositeResource
	// Ready indicated whether the composite resource should be marked as
	// ready or unready regardless of the state of the composed resoureces.
	// If it is nil the readiness of the composite is determined by the
	// readiness of the composed resources.
	Ready *bool
}

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
	// all of its readiness checks passed. Setting it to false will cause the
	// XR to be marked as not ready.
	Ready bool

	// Synced indicates whether the composition process was able to sync the
	// composed resource with its desired state. Setting it to false will cause
	// the XR to be marked as not synced.
	Synced bool
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

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

// ComposedResourceState tracks the state of a composed resource through the
// Composition process.
type ComposedResourceState struct {
	// State that is returned to the caller.
	ComposedResource

	// Things used to produce a composed resource.
	Template *v1.ComposedTemplate

	// The state of the composed resource.
	SuccessfullyRendered bool
	Resource             resource.Composed
	ConnectionDetails    managed.ConnectionDetails
}

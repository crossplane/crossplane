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

	iov1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// A ComposedResource is an output of the composition process.
type ComposedResource struct {
	// ResourceName identifies the composed resource within a Composition or
	// FunctionIO. This is not the metadata.name of the actual composed resource
	// instance; rather it is the name of an entry in a Composition's resources
	// array, and/or a FunctionIO's observed/desired resources array.
	ResourceName string

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
	Desired  *iov1alpha1.DesiredResource

	// The state of the composed resource.
	TemplateRenderErr error
	Resource          resource.Composed
	ConnectionDetails managed.ConnectionDetails
}

// ComposedResourceStates is a map of (Composition) resource name to state. The
// key corresponds to the ResourceName field of the ComposedResource type.
type ComposedResourceStates map[string]ComposedResourceState

// Merge the supplied composed resource state into the map of states. See
// MergeComposedResourceStates for details.
func (rs ComposedResourceStates) Merge(s ComposedResourceState) {
	rs[s.ResourceName] = MergeComposedResourceStates(rs[s.ResourceName], s)
}

// MergeComposedResourceStates merges the new ComposedResourceState into the old
// one. It is used to update the result of a P&T composition with the result of
// a subsequent function composition operation on the same composed resource.
func MergeComposedResourceStates(old, new ComposedResourceState) ComposedResourceState {
	out := old
	if new.ResourceName != "" {
		out.ResourceName = new.ResourceName
	}
	if new.Resource != nil {
		out.Resource = new.Resource
	}
	if new.Template != nil {
		out.Template = new.Template
	}
	if new.Desired != nil {
		out.Desired = new.Desired
	}
	// TODO(negz): Should Ready be *bool so we can differentiate between false
	// and unset? In practice we currently only ever transition _to_ these
	// states.
	if new.Ready {
		out.Ready = new.Ready
	}
	if new.TemplateRenderErr != nil {
		out.TemplateRenderErr = new.TemplateRenderErr
	}
	if out.ConnectionDetails == nil {
		out.ConnectionDetails = make(managed.ConnectionDetails)
	}
	for k, v := range new.ConnectionDetails {
		out.ConnectionDetails[k] = v
	}

	return out
}

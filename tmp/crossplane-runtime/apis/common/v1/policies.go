/*
Copyright 2019 The Crossplane Authors.

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

package v1

import (
	"github.com/crossplane/crossplane-runtime/v2/apis/common"
)

// ManagementPolicies determine how should Crossplane controllers manage an
// external resource through an array of ManagementActions.
type ManagementPolicies = common.ManagementPolicies

// A ManagementAction represents an action that the Crossplane controllers
// can take on an external resource.
type ManagementAction = common.ManagementAction

const (
	// ManagementActionObserve means that the managed resource status.atProvider
	// will be updated with the external resource state.
	ManagementActionObserve = common.ManagementActionObserve

	// ManagementActionCreate means that the external resource will be created
	// using the managed resource spec.initProvider and spec.forProvider.
	ManagementActionCreate = common.ManagementActionCreate

	// ManagementActionUpdate means that the external resource will be updated
	// using the managed resource spec.forProvider.
	ManagementActionUpdate = common.ManagementActionUpdate

	// ManagementActionDelete means that the external resource will be deleted
	// when the managed resource is deleted.
	ManagementActionDelete = common.ManagementActionDelete

	// ManagementActionLateInitialize means that unspecified fields of the managed
	// resource spec.forProvider will be updated with the external resource state.
	ManagementActionLateInitialize = common.ManagementActionLateInitialize

	// ManagementActionAll means that all of the above actions will be taken
	// by the Crossplane controllers.
	ManagementActionAll = common.ManagementActionAll
)

// A DeletionPolicy determines what should happen to the underlying external
// resource when a managed resource is deleted.
// +kubebuilder:validation:Enum=Orphan;Delete
type DeletionPolicy string

const (
	// DeletionOrphan means the external resource will be orphaned when its
	// managed resource is deleted.
	DeletionOrphan DeletionPolicy = "Orphan"

	// DeletionDelete means both the  external resource will be deleted when its
	// managed resource is deleted.
	DeletionDelete DeletionPolicy = "Delete"
)

// A CompositeDeletePolicy determines how the composite resource should be deleted
// when the corresponding claim is deleted.
type CompositeDeletePolicy = common.CompositeDeletePolicy

const (
	// CompositeDeleteBackground means the composite resource will be deleted using
	// the Background Propagation Policy when the claim is deleted.
	CompositeDeleteBackground = common.CompositeDeleteBackground

	// CompositeDeleteForeground means the composite resource will be deleted using
	// the Foreground Propagation Policy when the claim is deleted.
	CompositeDeleteForeground = common.CompositeDeleteForeground
)

// An UpdatePolicy determines how something should be updated - either
// automatically (without human intervention) or manually.
type UpdatePolicy = common.UpdatePolicy

const (
	// UpdateAutomatic means the resource should be updated automatically,
	// without any human intervention.
	UpdateAutomatic = common.UpdateAutomatic

	// UpdateManual means the resource requires human intervention to
	// update.
	UpdateManual = common.UpdateManual
)

// ResolvePolicy is a type for resolve policy.
type ResolvePolicy = common.ResolvePolicy

// ResolutionPolicy is a type for resolution policy.
type ResolutionPolicy = common.ResolutionPolicy

const (
	// ResolvePolicyAlways is a resolve option.
	// When the ResolvePolicy is set to ResolvePolicyAlways the reference will
	// be tried to resolve for every reconcile loop.
	ResolvePolicyAlways = common.ResolvePolicyAlways

	// ResolutionPolicyRequired is a resolution option.
	// When the ResolutionPolicy is set to ResolutionPolicyRequired the execution
	// could not continue even if the reference cannot be resolved.
	ResolutionPolicyRequired = common.ResolutionPolicyRequired

	// ResolutionPolicyOptional is a resolution option.
	// When the ReferenceResolutionPolicy is set to ReferencePolicyOptional the
	// execution could continue even if the reference cannot be resolved.
	ResolutionPolicyOptional = common.ResolutionPolicyOptional
)

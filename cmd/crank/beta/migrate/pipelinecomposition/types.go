package pipelinecomposition

import v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

// This struct is copied from function patch and transform, as we can't import it directly
// https://github.com/crossplane-contrib/function-patch-and-transform/blob/main/input/v1beta1/resources.go
type Input struct {
	// PatchSets define a named set of patches that may be included by any
	// resource in this Composition. PatchSets cannot themselves refer to other
	// PatchSets.
	//
	// PatchSets are only used by the "Resources" mode of Composition. They
	// are ignored by other modes.
	// +optional
	PatchSets []v1.PatchSet `json:"patchSets,omitempty"`

	// Environment configures the environment in which resources are rendered.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	Environment *v1.EnvironmentConfiguration `json:"environment,omitempty"`

	// Resources is a list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	//
	// Resources are only used by the "Resources" mode of Composition. They are
	// ignored by other modes.
	// +optional
	Resources []v1.ComposedTemplate `json:"resources,omitempty"`
}

// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Generated from apiextensions/v1/composition_revision_types.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

const (
	// LabelCompositionName is the name of the Composition used to create
	// this CompositionRevision.
	LabelCompositionName = "crossplane.io/composition-name"

	// LabelCompositionHash is a hash of the Composition label, annotation
	// and spec used to create this CompositionRevision. Used to identify
	// identical revisions.
	LabelCompositionHash = "crossplane.io/composition-hash"
)

// CompositionRevisionSpec specifies the desired state of the composition
// revision.
type CompositionRevisionSpec struct {
	// CompositeTypeRef specifies the type of composite resource that this
	// composition is compatible with.
	// +immutable
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// Mode controls what type or "mode" of Composition will be used.
	//
	// "Resources" (the default) indicates that a Composition uses what is
	// commonly referred to as "Patch & Transform" or P&T composition. This mode
	// of Composition uses an array of resources, each a template for a composed
	// resource.
	//
	// "Pipeline" indicates that a Composition specifies a pipeline
	// of Composition Functions, each of which is responsible for producing
	// composed resources that Crossplane should create or update. THE PIPELINE
	// MODE IS A BETA FEATURE. It is not honored if the relevant Crossplane
	// feature flag is disabled.
	// +optional
	// +kubebuilder:validation:Enum=Resources;Pipeline
	// +kubebuilder:default=Resources
	Mode *CompositionMode `json:"mode,omitempty"`

	// PatchSets define a named set of patches that may be included by any
	// resource in this Composition. PatchSets cannot themselves refer to other
	// PatchSets.
	//
	// PatchSets are only used by the "Resources" mode of Composition. They
	// are ignored by other modes.
	// +optional
	PatchSets []PatchSet `json:"patchSets,omitempty"`

	// Environment configures the environment in which resources are rendered.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	Environment *EnvironmentConfiguration `json:"environment,omitempty"`

	// Resources is a list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	//
	// Resources are only used by the "Resources" mode of Composition. They are
	// ignored by other modes.
	// +optional
	Resources []ComposedTemplate `json:"resources,omitempty"`

	// Pipeline is a list of composition function steps that will be used when a
	// composite resource referring to this composition is created. One of
	// resources and pipeline must be specified - you cannot specify both.
	//
	// The Pipeline is only used by the "Pipeline" mode of Composition. It is
	// ignored by other modes.
	//
	// THIS IS A BETA FIELD. It is not honored if the relevant Crossplane
	// feature flag is disabled.
	// +optional
	Pipeline []PipelineStep `json:"pipeline,omitempty"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// This field is planned to be replaced in a future release in favor of
	// PublishConnectionDetailsWithStoreConfigRef. Currently, both could be
	// set independently and connection details would be published to both
	// without affecting each other as long as related fields at MR level
	// specified.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`

	// PublishConnectionDetailsWithStoreConfig specifies the secret store config
	// with which the connection details of composite resources dynamically
	// provisioned using this composition will be published.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	// +kubebuilder:default={"name": "default"}
	PublishConnectionDetailsWithStoreConfigRef *StoreConfigReference `json:"publishConnectionDetailsWithStoreConfigRef,omitempty"`

	// Revision number. Newer revisions have larger numbers.
	// +immutable
	Revision int64 `json:"revision"`
}

// CompositionRevisionStatus shows the observed state of the composition
// revision.
type CompositionRevisionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A CompositionRevision represents a revision in time of a Composition.
// Revisions are created by Crossplane; they should be treated as immutable.
// +kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".spec.revision"
// +kubebuilder:printcolumn:name="XR-KIND",type="string",JSONPath=".spec.compositeTypeRef.kind"
// +kubebuilder:printcolumn:name="XR-APIVERSION",type="string",JSONPath=".spec.compositeTypeRef.apiVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=comprev
// +kubebuilder:subresource:status
type CompositionRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositionRevisionSpec   `json:"spec,omitempty"`
	Status CompositionRevisionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositionRevisionList contains a list of CompositionRevisions.
type CompositionRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompositionRevision `json:"items"`
}

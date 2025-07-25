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

package v1

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
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// Mode controls what type or "mode" of Composition will be used.
	//
	// "Pipeline" indicates that a Composition specifies a pipeline of
	// functions, each of which is responsible for producing composed
	// resources that Crossplane should create or update.
	//
	// +optional
	// +kubebuilder:validation:Enum=Pipeline
	// +kubebuilder:default=Pipeline
	Mode CompositionMode `json:"mode,omitempty"`

	// Pipeline is a list of function steps that will be used when a
	// composite resource referring to this composition is created.
	//
	// The Pipeline is only used by the "Pipeline" mode of Composition. It is
	// ignored by other modes.
	// +optional
	// +listType=map
	// +listMapKey=step
	Pipeline []PipelineStep `json:"pipeline,omitempty"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`

	// Revision number. Newer revisions have larger numbers.
	//
	// This number can change. When a Composition transitions from state A
	// -> B -> A there will be only two CompositionRevisions. Crossplane will
	// edit the original CompositionRevision to change its revision number from
	// 0 to 2.
	Revision int64 `json:"revision"`
}

// CompositionRevisionStatus shows the observed state of the composition
// revision.
type CompositionRevisionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A CompositionRevision represents a revision of a Composition. Crossplane
// creates new revisions when there are changes to the Composition.
//
// Crossplane creates and manages CompositionRevisions. Don't directly edit
// CompositionRevisions.
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

// GetCondition of this CompositionRevision.
func (in *CompositionRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return in.Status.GetCondition(ct)
}

// SetConditions of this CompositionRevision.
func (in *CompositionRevision) SetConditions(c ...xpv1.Condition) {
	in.Status.SetConditions(c...)
}

// +kubebuilder:object:root=true

// CompositionRevisionList contains a list of CompositionRevisions.
type CompositionRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositionRevision `json:"items"`
}

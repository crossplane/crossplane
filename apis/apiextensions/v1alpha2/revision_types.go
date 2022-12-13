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

package v1alpha2

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	// Composition is the manifest of the composition object that is stored by
	// this composition revision.
	Composition extv1.JSON `json:"composition"`

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
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A CompositionRevision represents a revision in time of a Composition.
// Revisions are created by Crossplane; they should be treated as immutable.
// +kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".spec.revision"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane
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

func init() {
	SchemeBuilder.Register(&CompositionRevision{}, &CompositionRevisionList{})
}

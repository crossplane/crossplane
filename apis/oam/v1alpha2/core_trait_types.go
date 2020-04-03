/*
Copyright 2020 The Crossplane Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A ManualScalerTraitSpec defines the desired state of a ManualScalerTrait.
type ManualScalerTraitSpec struct {
	// ReplicaCount of the workload this trait applies to.
	ReplicaCount int32 `json:"replicaCount"`

	// WorkloadReference to the workload this trait applies to.
	WorkloadReference runtimev1alpha1.TypedReference `json:"workloadRef"`
}

// A ManualScalerTraitStatus represents the observed state of a
// ManualScalerTrait.
type ManualScalerTraitStatus struct {
	runtimev1alpha1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ManualScalerTrait determines how many replicas a workload should have.
// +kubebuilder:resource:categories={crossplane,oam}
// +kubebuilder:subresource:status
type ManualScalerTrait struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManualScalerTraitSpec   `json:"spec,omitempty"`
	Status ManualScalerTraitStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManualScalerTraitList contains a list of ManualScalerTrait.
type ManualScalerTraitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManualScalerTrait `json:"items"`
}

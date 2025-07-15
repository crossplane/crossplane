/*
Copyright 2025 The Crossplane Authors.

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

package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ActivationPolicy matches on MRD names with wildcard prefix support.
type ActivationPolicy string

// ManagedResourceActivationPolicySpec specifies the desired activation state of ManagedResourceDefinitions.
type ManagedResourceActivationPolicySpec struct {
	// Activations is an array of MRD names to activate. Supports wildcard
	// prefixes (like `*.aws.crossplane.io`) but not full regular expressions.
	// +required
	Activations []ActivationPolicy `json:"activate"`
}

// ManagedResourceActivationPolicyStatus shows the observed state of the policy.
type ManagedResourceActivationPolicyStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// Activated names the ManagedResourceDefinitions this policy has activated.
	// +optional
	Activated []string `json:"activated,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A ManagedResourceActivationPolicy defines the activation policy for ManagedResourceDefinitions.
//
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=mrap
type ManagedResourceActivationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedResourceActivationPolicySpec   `json:"spec,omitempty"`
	Status ManagedResourceActivationPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedResourceActivationPolicyList contains a list of ManagedResourceActivationPolicy.
type ManagedResourceActivationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ManagedResourceActivationPolicy `json:"items"`
}

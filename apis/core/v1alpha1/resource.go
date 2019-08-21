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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// +kubebuilder:object:root=true

// ResourceClass is the Schema for the instances API
// +kubebuilder:printcolumn:name="PROVISIONER",type="string",JSONPath=".provisioner"
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ResourceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Parameters holds parameters for the provisioner.
	// These values are opaque to the  system and are passed directly
	// to the provisioner.  The only validation done on keys is that they are
	// not empty.  The maximum number of parameters is
	// 512, with a cumulative max size of 256K
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// Provisioner is the driver expected to handle this ResourceClass.
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.crossplane.io/v1alpha1" or "CloudSQLInstance.database.gcp.crossplane.io/v1alpha1".
	// This value may not be empty.
	Provisioner string `json:"provisioner"`

	// ProviderReference is a reference to the cloud provider that will be used
	// to provision the concrete cloud resource.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// reclaimPolicy is the reclaim policy that dynamically provisioned
	// ResourceInstances of this resource class are created with
	// +optional
	ReclaimPolicy runtimev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// NOTE(hasheddan): cannot check for satisfying resource.Class interface
// because of circular imports

// GetReclaimPolicy of this ResourceClass.
func (r *ResourceClass) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return r.ReclaimPolicy
}

// SetReclaimPolicy of this ResourceClass.
func (r *ResourceClass) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	r.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// ResourceClassList contains a list of resource classes.
type ResourceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceClass `json:"items"`
}

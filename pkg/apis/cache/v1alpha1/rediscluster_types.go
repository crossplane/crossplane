/*
Copyright 2018 The Crossplane Authors.

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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// RedisClusterSpec defines the desired state of RedisCluster
type RedisClusterSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// EngineVersion specifies the desired Redis version.
	// +kubebuilder:validation:Enum=2.6,2.8,3.2,4.0,5.0
	EngineVersion string `json:"engineVersion"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisCluster is the the CRD type for abstract Redis clusters. Crossplane
// considers a single Redis instance a 'cluster' of one instance.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec                 `json:"spec,omitempty"`
	Status corev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisClusterList contains a list of RedisCluster
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

// ClaimStatus returns the status of this resource claim
func (c *RedisCluster) ClaimStatus() *corev1alpha1.ResourceClaimStatus {
	return &c.Status
}

// ClassRef return the reference to the resource class this claim uses.
func (c *RedisCluster) ClassRef() *corev1.ObjectReference {
	return c.Spec.ClassRef
}

// ResourceRef returns the reference to the resource this claim is bound to.
func (c *RedisCluster) ResourceRef() *corev1.ObjectReference {
	return c.Spec.ResourceRef
}

// SetResourceRef sets the reference to the resource this claim is bound to.
func (c *RedisCluster) SetResourceRef(ref *corev1.ObjectReference) {
	c.Spec.ResourceRef = ref
}

func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}

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

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// RedisClusterSpec defines the desired state of a RedisCluster.
type RedisClusterSpec struct {
	v1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired Redis version.
	// +kubebuilder:validation:Enum=2.6,2.8,3.2,4.0,5.0
	EngineVersion string `json:"engineVersion"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisCluster is the the CRD type for abstract Redis clusters. Crossplane
// considers a single Redis instance a 'cluster' of one instance.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec             `json:"spec,omitempty"`
	Status v1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisClusterList contains a list of RedisCluster
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

// SetBindingPhase sets the binding phase of the RedisCluster.
func (rc *RedisCluster) SetBindingPhase(p v1alpha1.BindingPhase) {
	rc.Status.SetBindingPhase(p)
}

// GetBindingPhase gets the binding phase of the RedisCluster.
func (rc *RedisCluster) GetBindingPhase() v1alpha1.BindingPhase {
	return rc.Status.GetBindingPhase()
}

// SetConditions on this redis cluster.
func (rc *RedisCluster) SetConditions(c ...v1alpha1.Condition) {
	rc.Status.SetConditions(c...)
}

// SetClassReference to this RedisCluster's resource class.
func (rc *RedisCluster) SetClassReference(r *corev1.ObjectReference) {
	rc.Spec.ClassReference = r
}

// GetClassReference returns this RedisCluster's resource class.
func (rc *RedisCluster) GetClassReference() *corev1.ObjectReference {
	return rc.Spec.ClassReference
}

// SetResourceReference to this RedisCluster's allocated managed resource.
func (rc *RedisCluster) SetResourceReference(r *corev1.ObjectReference) {
	rc.Spec.ResourceReference = r
}

// GetResourceReference returns this RedisCluster's allocated managed resource.
func (rc *RedisCluster) GetResourceReference() *corev1.ObjectReference {
	return rc.Spec.ResourceReference
}

// SetWriteConnectionSecretTo sets the connection secret this RedisCluster
// should write its connection secret to.
func (rc *RedisCluster) SetWriteConnectionSecretTo(r corev1.LocalObjectReference) {
	rc.Spec.WriteConnectionSecretTo = r
}

// GetWriteConnectionSecretTo returns the connection secret this RedisCluster
// will write its connection secret to.
func (rc *RedisCluster) GetWriteConnectionSecretTo() corev1.LocalObjectReference {
	return rc.Spec.WriteConnectionSecretTo
}

func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}

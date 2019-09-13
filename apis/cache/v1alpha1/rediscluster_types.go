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
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

// RedisClusterSpec specifies the desired state of a RedisCluster.
type RedisClusterSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired Redis version.
	// +kubebuilder:validation:Enum="2.6";"2.8";"3.2";"4.0";"5.0"
	EngineVersion string `json:"engineVersion,omitempty"`
}

var _ resource.Claim = &RedisCluster{}

// +kubebuilder:object:root=true

// A RedisCluster is a portable resource claim that may be satisfied by binding
// to a Redis managed resource such as a GCP CloudMemorystore instance or an AWS
// ReplicationGroup. Despite the name RedisCluster claims may bind to Redis
// managed resources that are a single node, or not in cluster mode.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec                    `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this RedisCluster.
func (rc *RedisCluster) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	rc.Status.SetBindingPhase(p)
}

// GetBindingPhase of this RedisCluster.
func (rc *RedisCluster) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return rc.Status.GetBindingPhase()
}

// SetConditions of this RedisCluster.
func (rc *RedisCluster) SetConditions(c ...runtimev1alpha1.Condition) {
	rc.Status.SetConditions(c...)
}

// SetPortableClassReference of this RedisCluster.
func (rc *RedisCluster) SetPortableClassReference(r *corev1.LocalObjectReference) {
	rc.Spec.PortableClassReference = r
}

// GetPortableClassReference of this RedisCluster.
func (rc *RedisCluster) GetPortableClassReference() *corev1.LocalObjectReference {
	return rc.Spec.PortableClassReference
}

// SetResourceReference of this RedisCluster.
func (rc *RedisCluster) SetResourceReference(r *corev1.ObjectReference) {
	rc.Spec.ResourceReference = r
}

// GetResourceReference of this RedisCluster.
func (rc *RedisCluster) GetResourceReference() *corev1.ObjectReference {
	return rc.Spec.ResourceReference
}

// SetWriteConnectionSecretToReference of this RedisCluster.
func (rc *RedisCluster) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	rc.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this RedisCluster.
func (rc *RedisCluster) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return rc.Spec.WriteConnectionSecretToReference
}

// +kubebuilder:object:root=true

// RedisClusterList contains a list of RedisCluster.
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

// All portable classes must satisfy the PortableClass interface
var _ resource.PortableClass = &RedisClusterClass{}

// +kubebuilder:object:root=true

// RedisClusterClass contains a namespace-scoped portable class for RedisCluster
type RedisClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.PortableClass `json:",inline"`
}

// All portable class lists must satisfy the PortableClassList interface
var _ resource.PortableClassList = &RedisClusterClassList{}

// +kubebuilder:object:root=true

// RedisClusterClassList contains a list of RedisClusterClass.
type RedisClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisClusterClass `json:"items"`
}

// SetPortableClassItems of this RedisClusterClassList.
func (rc *RedisClusterClassList) SetPortableClassItems(r []resource.PortableClass) {
	items := make([]RedisClusterClass, 0, len(r))
	for i := range r {
		if item, ok := r[i].(*RedisClusterClass); ok {
			items = append(items, *item)
		}
	}
	rc.Items = items
}

// GetPortableClassItems of this RedisClusterClassList.
func (rc *RedisClusterClassList) GetPortableClassItems() []resource.PortableClass {
	items := make([]resource.PortableClass, len(rc.Items))
	for i, item := range rc.Items {
		item := item
		items[i] = resource.PortableClass(&item)
	}
	return items
}

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

// KubernetesClusterSpec specifies the desired state of a KubernetesCluster.
type KubernetesClusterSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// ClusterVersion specifies the desired Kubernetes version, e.g. 1.15.
	ClusterVersion string `json:"clusterVersion,omitempty"`
}

var _ resource.Claim = &KubernetesCluster{}

// +kubebuilder:object:root=true

// A KubernetesCluster is a portable resource claim that may be satisfied by
// binding to a Kubernetes cluster managed resource such as an AWS EKS cluster
// or an Azure AKS cluster.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="CLUSTER-REF",type="string",JSONPath=".spec.resourceName.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec               `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this KubernetesCluster.
func (kc *KubernetesCluster) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	kc.Status.SetBindingPhase(p)
}

// GetBindingPhase of this KubernetesCluster.
func (kc *KubernetesCluster) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return kc.Status.GetBindingPhase()
}

// SetConditions of this KubernetesCluster.
func (kc *KubernetesCluster) SetConditions(c ...runtimev1alpha1.Condition) {
	kc.Status.SetConditions(c...)
}

// SetPortableClassReference of this KubernetesCluster.
func (kc *KubernetesCluster) SetPortableClassReference(r *corev1.LocalObjectReference) {
	kc.Spec.PortableClassReference = r
}

// GetPortableClassReference of this KubernetesCluster.
func (kc *KubernetesCluster) GetPortableClassReference() *corev1.LocalObjectReference {
	return kc.Spec.PortableClassReference
}

// SetResourceReference of this KubernetesCluster.
func (kc *KubernetesCluster) SetResourceReference(r *corev1.ObjectReference) {
	kc.Spec.ResourceReference = r
}

// GetResourceReference of this KubernetesCluster.
func (kc *KubernetesCluster) GetResourceReference() *corev1.ObjectReference {
	return kc.Spec.ResourceReference
}

// SetWriteConnectionSecretToReference of this KubernetesCluster.
func (kc *KubernetesCluster) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	kc.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this KubernetesCluster.
func (kc *KubernetesCluster) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return kc.Spec.WriteConnectionSecretToReference
}

// +kubebuilder:object:root=true

// KubernetesClusterList contains a list of KubernetesCluster.
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

// All portable classes must satisfy the Class interface
var _ resource.PortableClass = &KubernetesClusterClass{}

// +kubebuilder:object:root=true

// KubernetesClusterClass contains a namespace-scoped Class for KubernetesCluster
type KubernetesClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.PortableClass `json:",inline"`
}

// All portable class lists must satisfy the ClassList interface
var _ resource.PortableClassList = &KubernetesClusterClassList{}

// +kubebuilder:object:root=true

// KubernetesClusterClassList contains a list of KubernetesClusterClass.
type KubernetesClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesClusterClass `json:"items"`
}

// SetPortableClassItems of this KubernetesClusterClassList.
func (kc *KubernetesClusterClassList) SetPortableClassItems(r []resource.PortableClass) {
	items := make([]KubernetesClusterClass, 0, len(r))
	for i := range r {
		if item, ok := r[i].(*KubernetesClusterClass); ok {
			items = append(items, *item)
		}
	}
	kc.Items = items
}

// GetPortableClassItems of this KubernetesClusterClassList.
func (kc *KubernetesClusterClassList) GetPortableClassItems() []resource.PortableClass {
	items := make([]resource.PortableClass, len(kc.Items))
	for i, item := range kc.Items {
		item := item
		items[i] = resource.PortableClass(&item)
	}
	return items
}

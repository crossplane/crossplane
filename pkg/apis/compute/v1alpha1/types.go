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

// KubernetesClusterSpec specifies the configuration of a Kubernetes cluster.
type KubernetesClusterSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// cluster properties
	ClusterVersion string `json:"clusterVersion,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesCluster is the Schema for the instances API
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="CLUSTER-REF",type="string",JSONPath=".spec.resourceName.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec            `json:"spec,omitempty"`
	Status corev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesClusterList contains a list of KubernetesClusters.
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

// ClaimStatus returns the claim status of this Kubernetes cluster.
func (kc *KubernetesCluster) ClaimStatus() *corev1alpha1.ResourceClaimStatus {
	return &kc.Status
}

// ClassRef returns the resource class used by this Kubernetes cluster.
func (kc *KubernetesCluster) ClassRef() *corev1.ObjectReference {
	return kc.Spec.ClassRef
}

// ResourceRef returns the resource claimed by this Kubernetes cluster.
func (kc *KubernetesCluster) ResourceRef() *corev1.ObjectReference {
	return kc.Spec.ResourceRef
}

// SetResourceRef sets the resource claimed by this Kubernetes cluster.
func (kc *KubernetesCluster) SetResourceRef(ref *corev1.ObjectReference) {
	kc.Spec.ResourceRef = ref
}

// ResourceReference is generic resource represented by the resource name and the secret name that will be generated
// for the consumption inside the Workload.
// TODO: Note, currently resource reference is a general type, however, this will be change in the future and replaced with concrete resource types
type ResourceReference struct {
	// reference to a resource object in the same namespace
	corev1.ObjectReference `json:",inline"`
	// name of the generated resource secret
	SecretName string `json:"secretName"`
}

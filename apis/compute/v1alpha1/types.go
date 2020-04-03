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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// KubernetesClusterSpec specifies the desired state of a KubernetesCluster.
type KubernetesClusterSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// ClusterVersion specifies the desired Kubernetes version, e.g. 1.15.
	ClusterVersion string `json:"clusterVersion,omitempty"`
}

// +kubebuilder:object:root=true

// A KubernetesCluster is a portable resource claim that may be satisfied by
// binding to a Kubernetes cluster managed resource such as an AWS EKS cluster
// or an Azure AKS cluster.
// +kubebuilder:resource:categories={crossplane,claim}
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS-KIND",type="string",JSONPath=".spec.classRef.kind"
// +kubebuilder:printcolumn:name="CLASS-NAME",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".spec.resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".spec.resourceRef.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec               `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesClusterList contains a list of KubernetesCluster.
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

// MachineInstanceSpec specifies the desired state of a MachineInstance.
type MachineInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`
}

// +kubebuilder:object:root=true

// A MachineInstance is a portable resource claim that may be satisfied by
// binding to a machine instance, which may include Virtual Machine managed
// resources such as an AWS EC2 instance or bare metal managed resources such as
// a Packet Device.
// +kubebuilder:resource:categories={crossplane,claim}
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS-KIND",type="string",JSONPath=".spec.classRef.kind"
// +kubebuilder:printcolumn:name="CLASS-NAME",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".spec.resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".spec.resourceRef.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type MachineInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineInstanceSpec                 `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineInstanceList contains a list of Instance.
type MachineInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineInstance `json:"items"`
}

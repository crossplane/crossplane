/*
Copyright 2018 The Conductor Authors.

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
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//----------------------------------------------------------------------------------------------------------------------

// KubernetesClusterSpec
type KubernetesClusterSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// cluster properties
	ClusterVersion string `json:"clusterVersion,omitempty"`
}

// KubernetesClusterStatus
type KubernetesClusterStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resource
	// This is an optionally-prefixed name, like a label key.
	// For example: "EKScluster.compute.aws.conductor.io/v1alpha1" or "GKECluster.compute.gcp.conductor.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesCluster is the Schema for the instances API
// +k8s:openapi-gen=true
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec   `json:"spec,omitempty"`
	Status KubernetesClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesClusterList contains a list of RDSInstance
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

// ObjectReference to using this object as a reference
func (kc *KubernetesCluster) ObjectReference() *corev1.ObjectReference {
	if kc.Kind == "" {
		kc.Kind = KubernetesInstanceKind
	}
	if kc.APIVersion == "" {
		kc.APIVersion = APIVersion
	}
	return &corev1.ObjectReference{
		APIVersion: kc.APIVersion,
		Kind:       kc.Kind,
		Name:       kc.Name,
		Namespace:  kc.Namespace,
		UID:        kc.UID,
	}
}

// OwnerReference to use this object as an owner
func (kc *KubernetesCluster) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(kc.ObjectReference())
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

// WorkloadSpec
type WorkloadSpec struct {
	TargetNamespace  string             `json:"targetNamespace,omitempty"`
	TargetDeployment *appsv1.Deployment `json:"targetDeployment,omitempty"`
	TargetService    *corev1.Service    `json:"targetService,omitempty"`

	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// cluster properties
	ClusterVersion string `json:"clusterVersion,omitempty"`

	// Resources
	Resources []ResourceReference `json:"resources"`
}

// WorkloadStatus
type WorkloadStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resource
	// This is an optionally-prefixed name, like a label key.
	// For example: "EKScluster.compute.aws.conductor.io/v1alpha1" or "GKECluster.compute.gcp.conductor.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workload is the Schema for the instances API
// +k8s:openapi-gen=true
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSpec   `json:"spec,omitempty"`
	Status WorkloadStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadList contains a list of RDSInstance
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}

// ObjectReference to using this object as a reference
func (kc *Workload) ObjectReference() *corev1.ObjectReference {
	if kc.Kind == "" {
		kc.Kind = KubernetesInstanceKind
	}
	if kc.APIVersion == "" {
		kc.APIVersion = APIVersion
	}
	return &corev1.ObjectReference{
		APIVersion: kc.APIVersion,
		Kind:       kc.Kind,
		Name:       kc.Name,
		Namespace:  kc.Namespace,
		UID:        kc.UID,
	}
}

// OwnerReference to use this object as an owner
func (kc *Workload) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(kc.ObjectReference())
}

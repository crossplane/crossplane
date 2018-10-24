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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//----------------------------------------------------------------------------------------------------------------------

// MySQLInstanceSpec
type MySQLInstanceSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// mysql instance properties
	EngineVersion string `json:"engineVersion,omitempty"`
}

// MySQLInstanceClaimStatus
type MySQLInstanceClaimStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resrouce
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.conductor.io/v1alpha1" or "CloudSQLInstance.database.gcp.conductor.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstance is the Schema for the instances API
// +k8s:openapi-gen=true
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec        `json:"spec,omitempty"`
	Status MySQLInstanceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstanceList contains a list of RDSInstance
type MySQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstance `json:"items"`
}

// ObjectReference to using this object as a reference
func (m *MySQLInstance) ObjectReference() *corev1.ObjectReference {
	if m.Kind == "" {
		m.Kind = MySQLInstanceKind
	}
	if m.APIVersion == "" {
		m.APIVersion = APIVersion
	}
	return &corev1.ObjectReference{
		APIVersion: m.APIVersion,
		Kind:       m.Kind,
		Name:       m.Name,
		Namespace:  m.Namespace,
		UID:        m.UID,
	}
}

// OwnerReference to use this object as an owner
func (m *MySQLInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(m.ObjectReference())
}

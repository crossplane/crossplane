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

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// MySQLInstanceSpec specifies the desired state of a MySQLInstance.
type MySQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired MySQL engine version, e.g. 5.7.
	// +kubebuilder:validation:Enum="5.6";"5.7"
	EngineVersion string `json:"engineVersion,omitempty"`
}

// +kubebuilder:object:root=true

// A MySQLInstance is a portable resource claim that may be satisfied by binding
// to a MySQL managed resource such as an AWS RDS instance or a GCP CloudSQL
// instance.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS-KIND",type="string",JSONPath=".spec.classRef.kind"
// +kubebuilder:printcolumn:name="CLASS-NAME",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".spec.resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".spec.resourceRef.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec                   `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MySQLInstanceList contains a list of MySQLInstance.
type MySQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstance `json:"items"`
}

// PostgreSQLInstanceSpec specifies the desired state of a PostgreSQLInstance.
// PostgreSQLInstance.
type PostgreSQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired PostgreSQL engine version, e.g. 9.6.
	// +kubebuilder:validation:Enum="9.6"
	EngineVersion string `json:"engineVersion,omitempty"`
}

// +kubebuilder:object:root=true

// A PostgreSQLInstance is a portable resource claim that may be satisfied by
// binding to a PostgreSQL managed resource such as an AWS RDS instance or a GCP
// CloudSQL instance.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS-KIND",type="string",JSONPath=".spec.classRef.kind"
// +kubebuilder:printcolumn:name="CLASS-NAME",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".spec.resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".spec.resourceRef.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type PostgreSQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLInstanceSpec              `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgreSQLInstanceList contains a list of PostgreSQLInstance.
type PostgreSQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstance `json:"items"`
}

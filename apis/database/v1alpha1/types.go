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

// MySQLInstanceSpec specifies the desired state of a MySQLInstance.
type MySQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired MySQL engine version, e.g. 5.7.
	// +kubebuilder:validation:Enum="5.6";"5.7"
	EngineVersion string `json:"engineVersion,omitempty"`
}

var _ resource.Claim = &MySQLInstance{}

// +kubebuilder:object:root=true

// A MySQLInstance is a portable resource claim that may be satisfied by binding
// to a MySQL managed resource such as an AWS RDS instance or a GCP CloudSQL
// instance.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec                   `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this MySQLInstance.
func (i *MySQLInstance) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	i.Status.SetBindingPhase(p)
}

// GetBindingPhase of this MySQLInstance.
func (i *MySQLInstance) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return i.Status.GetBindingPhase()
}

// SetConditions of this MySQLInstance.
func (i *MySQLInstance) SetConditions(c ...runtimev1alpha1.Condition) {
	i.Status.SetConditions(c...)
}

// SetPortableClassReference of this MySQLInstance.
func (i *MySQLInstance) SetPortableClassReference(r *corev1.LocalObjectReference) {
	i.Spec.PortableClassReference = r
}

// GetPortableClassReference of this MySQLInstance.
func (i *MySQLInstance) GetPortableClassReference() *corev1.LocalObjectReference {
	return i.Spec.PortableClassReference
}

// SetResourceReference of this MySQLInstance.
func (i *MySQLInstance) SetResourceReference(r *corev1.ObjectReference) {
	i.Spec.ResourceReference = r
}

// GetResourceReference of this MySQLInstance.
func (i *MySQLInstance) GetResourceReference() *corev1.ObjectReference {
	return i.Spec.ResourceReference
}

// SetWriteConnectionSecretToReference of this MySQLInstance.
func (i *MySQLInstance) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	i.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this MySQLInstance.
func (i *MySQLInstance) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return i.Spec.WriteConnectionSecretToReference
}

// +kubebuilder:object:root=true

// MySQLInstanceList contains a list of MySQLInstance.
type MySQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstance `json:"items"`
}

// All portable classes must satisfy the PortableClass interface
var _ resource.PortableClass = &MySQLInstanceClass{}

// +kubebuilder:object:root=true

// MySQLInstanceClass contains a namespace-scoped portable class for MySQLInstance
type MySQLInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.PortableClass `json:",inline"`
}

// All portable class lists must satisfy the PortableClassList interface
var _ resource.PortableClassList = &MySQLInstanceClassList{}

// +kubebuilder:object:root=true

// MySQLInstanceClassList contains a list of MySQLInstanceClass.
type MySQLInstanceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstanceClass `json:"items"`
}

// SetPortableClassItems of this MySQLInstanceClassList.
func (my *MySQLInstanceClassList) SetPortableClassItems(r []resource.PortableClass) {
	items := make([]MySQLInstanceClass, 0, len(r))
	for i := range r {
		if item, ok := r[i].(*MySQLInstanceClass); ok {
			items = append(items, *item)
		}
	}
	my.Items = items
}

// GetPortableClassItems of this MySQLInstanceClassList.
func (my *MySQLInstanceClassList) GetPortableClassItems() []resource.PortableClass {
	items := make([]resource.PortableClass, len(my.Items))
	for i, item := range my.Items {
		item := item
		items[i] = resource.PortableClass(&item)
	}
	return items
}

// PostgreSQLInstanceSpec specifies the desired state of a PostgreSQLInstance.
// PostgreSQLInstance.
type PostgreSQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// EngineVersion specifies the desired PostgreSQL engine version, e.g. 9.6.
	// +kubebuilder:validation:Enum="9.6"
	EngineVersion string `json:"engineVersion,omitempty"`
}

var _ resource.Claim = &PostgreSQLInstance{}

// +kubebuilder:object:root=true

// A PostgreSQLInstance is a portable resource claim that may be satisfied by
// binding to a PostgreSQL managed resource such as an AWS RDS instance or a GCP
// CloudSQL instance.
// PostgreSQLInstance is the CRD type for abstract PostgreSQL database instances
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type PostgreSQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLInstanceSpec              `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	i.Status.SetBindingPhase(p)
}

// GetBindingPhase of this PostgreSQLInstance.
func (i *PostgreSQLInstance) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return i.Status.GetBindingPhase()
}

// SetConditions of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetConditions(c ...runtimev1alpha1.Condition) {
	i.Status.SetConditions(c...)
}

// SetPortableClassReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetPortableClassReference(r *corev1.LocalObjectReference) {
	i.Spec.PortableClassReference = r
}

// GetPortableClassReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) GetPortableClassReference() *corev1.LocalObjectReference {
	return i.Spec.PortableClassReference
}

// SetResourceReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetResourceReference(r *corev1.ObjectReference) {
	i.Spec.ResourceReference = r
}

// GetResourceReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) GetResourceReference() *corev1.ObjectReference {
	return i.Spec.ResourceReference
}

// SetWriteConnectionSecretToReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	i.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return i.Spec.WriteConnectionSecretToReference
}

// +kubebuilder:object:root=true

// PostgreSQLInstanceList contains a list of PostgreSQLInstance.
type PostgreSQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstance `json:"items"`
}

// All portable classes must satisfy the PortableClass interface
var _ resource.PortableClass = &PostgreSQLInstanceClass{}

// +kubebuilder:object:root=true

// PostgreSQLInstanceClass contains a namespace-scoped portable class for PostgreSQLInstance
type PostgreSQLInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.PortableClass `json:",inline"`
}

// All portable class lists must satisfy the PortableClassList interface
var _ resource.PortableClassList = &PostgreSQLInstanceClassList{}

// +kubebuilder:object:root=true

// PostgreSQLInstanceClassList contains a list of PostgreSQLInstanceClass.
type PostgreSQLInstanceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstanceClass `json:"items"`
}

// SetPortableClassItems of this PostgreSQLInstanceClassList.
func (pg *PostgreSQLInstanceClassList) SetPortableClassItems(r []resource.PortableClass) {
	items := make([]PostgreSQLInstanceClass, 0, len(r))
	for i := range r {
		if item, ok := r[i].(*PostgreSQLInstanceClass); ok {
			items = append(items, *item)
		}
	}
	pg.Items = items
}

// GetPortableClassItems of this PostgreSQLInstanceClassList.
func (pg *PostgreSQLInstanceClassList) GetPortableClassItems() []resource.PortableClass {
	items := make([]resource.PortableClass, len(pg.Items))
	for i, item := range pg.Items {
		item := item
		items[i] = resource.PortableClass(&item)
	}
	return items
}

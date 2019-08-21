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

// MySQLInstanceSpec specifies the configuration of a MySQL instance.
type MySQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// mysql instance properties
	// +kubebuilder:validation:Enum="5.6";"5.7"
	EngineVersion string `json:"engineVersion"`
}

// +kubebuilder:object:root=true

// MySQLInstance is the CRD type for abstract MySQL database instances
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

// SetClassReference of this MySQLInstance.
func (i *MySQLInstance) SetClassReference(r *corev1.ObjectReference) {
	i.Spec.ClassReference = r
}

// GetClassReference of this MySQLInstance.
func (i *MySQLInstance) GetClassReference() *corev1.ObjectReference {
	return i.Spec.ClassReference
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

// MySQLInstanceList contains a list of MySQLInstance
type MySQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstance `json:"items"`
}

// All policies must satisfy the Policy interface
var _ resource.Policy = &MySQLInstancePolicy{}

// +kubebuilder:object:root=true

// MySQLInstancePolicy contains a namespace-scoped policy for MySQLInstance
type MySQLInstancePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.Policy `json:",inline"`
}

// All policy lists must satisfy the PolicyList interface
var _ resource.PolicyList = &MySQLInstancePolicyList{}

// +kubebuilder:object:root=true

// MySQLInstancePolicyList contains a list of MySQLInstancePolicy
type MySQLInstancePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstancePolicy `json:"items"`
}

// PostgreSQLInstanceSpec specifies the configuration of this
// PostgreSQLInstance.
type PostgreSQLInstanceSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// postgresql instance properties
	// +kubebuilder:validation:Enum="9.6"
	EngineVersion string `json:"engineVersion,omitempty"`
}

// +kubebuilder:object:root=true

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

// SetClassReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) SetClassReference(r *corev1.ObjectReference) {
	i.Spec.ClassReference = r
}

// GetClassReference of this PostgreSQLInstance.
func (i *PostgreSQLInstance) GetClassReference() *corev1.ObjectReference {
	return i.Spec.ClassReference
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

// PostgreSQLInstanceList contains a list of PostgreSQLInstance
type PostgreSQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstance `json:"items"`
}

// All policies must satisfy the Policy interface
var _ resource.Policy = &PostgreSQLInstancePolicy{}

// +kubebuilder:object:root=true

// PostgreSQLInstancePolicy contains a namespace-scoped policy for PostgreSQLInstance
type PostgreSQLInstancePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.Policy `json:",inline"`
}

// All policy lists must satisfy the PolicyList interface
var _ resource.PolicyList = &PostgreSQLInstancePolicyList{}

// +kubebuilder:object:root=true

// PostgreSQLInstancePolicyList contains a list of PostgreSQLInstancePolicy
type PostgreSQLInstancePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstancePolicy `json:"items"`
}

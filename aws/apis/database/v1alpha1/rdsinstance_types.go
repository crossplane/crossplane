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
	"github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SQL database engines.
const (
	MysqlEngine      = "mysql"
	PostgresqlEngine = "postgres"
)

// RDSInstanceParameters defines the desired state of RDSInstance
type RDSInstanceParameters struct {
	MasterUsername string `json:"masterUsername"`
	Engine         string `json:"engine"`
	EngineVersion  string `json:"engineVersion,omitempty"`
	Class          string `json:"class"` // like "db.t2.micro"
	Size           int64  `json:"size"`  // size in gb

	// Specifies a DB subnet group for the DB instance. The new DB instance is created
	// in the VPC associated with the DB subnet group. If no DB subnet group is
	// specified, then the new DB instance is not created in a VPC.
	SubnetGroupName string `json:"subnetGroupName,omitempty"`

	// VPC Security groups that will allow the RDS instance to be accessed over the network.
	// You can consider the following groups:
	// 1) A default group that allows all communication amongst instances in that group
	// 2) A RDS specific group that allows port 3306 from allowed sources (clients and instances
	//	  that are expected to connect to the database.
	SecurityGroups []string `json:"securityGroups,omitempty"`
}

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
	v1alpha1.ResourceSpec `json:",inline"`
	RDSInstanceParameters `json:",inline"`
}

// RDSInstanceState represents the state of an RDS instance.
type RDSInstanceState string

// RDS instance states.
const (
	// The instance is healthy and available
	RDSInstanceStateAvailable RDSInstanceState = "available"
	// The instance is being created. The instance is inaccessible while it is being created.
	RDSInstanceStateCreating RDSInstanceState = "creating"
	// The instance is being deleted.
	RDSInstanceStateDeleting RDSInstanceState = "deleting"
	// The instance has failed and Amazon RDS can't recover it. Perform a point-in-time restore to the latest restorable time of the instance to recover the data.
	RDSInstanceStateFailed RDSInstanceState = "failed"
)

// RDSInstanceStatus defines the observed state of RDSInstance
type RDSInstanceStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`

	State        string `json:"state,omitempty"`
	Message      string `json:"message,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`   // the external ID to identify this resource in the cloud provider
	InstanceName string `json:"instanceName,omitempty"` // the generated DB Instance name
	Endpoint     string `json:"endpoint,omitempty"`     // rds instance endpoint
}

// +kubebuilder:object:root=true

// RDSInstance is the Schema for the instances API
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type RDSInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSInstanceSpec   `json:"spec,omitempty"`
	Status RDSInstanceStatus `json:"status,omitempty"`
}

// SetBindingPhase of this RDSInstance.
func (i *RDSInstance) SetBindingPhase(p v1alpha1.BindingPhase) {
	i.Status.SetBindingPhase(p)
}

// GetBindingPhase of this RDSInstance.
func (i *RDSInstance) GetBindingPhase() v1alpha1.BindingPhase {
	return i.Status.GetBindingPhase()
}

// SetConditions of this RDSInstance.
func (i *RDSInstance) SetConditions(c ...v1alpha1.Condition) {
	i.Status.SetConditions(c...)
}

// SetClaimReference of this RDSInstance.
func (i *RDSInstance) SetClaimReference(r *corev1.ObjectReference) {
	i.Spec.ClaimReference = r
}

// GetClaimReference of this RDSInstance.
func (i *RDSInstance) GetClaimReference() *corev1.ObjectReference {
	return i.Spec.ClaimReference
}

// SetClassReference of this RDSInstance.
func (i *RDSInstance) SetClassReference(r *corev1.ObjectReference) {
	i.Spec.ClassReference = r
}

// GetClassReference of this RDSInstance.
func (i *RDSInstance) GetClassReference() *corev1.ObjectReference {
	return i.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this RDSInstance.
func (i *RDSInstance) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	i.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this RDSInstance.
func (i *RDSInstance) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return i.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this RDSInstance.
func (i *RDSInstance) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return i.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this RDSInstance.
func (i *RDSInstance) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	i.Spec.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// RDSInstanceList contains a list of RDSInstance
type RDSInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstance `json:"items"`
}

// RDSInstanceClassSpecTemplate is the Schema for the resource class
type RDSInstanceClassSpecTemplate struct {
	v1alpha1.ResourceClassSpecTemplate `json:",inline"`
	RDSInstanceParameters              `json:",inline"`
}

var _ resource.Class = &RDSInstanceClass{}

// +kubebuilder:object:root=true

// RDSInstanceClass is the Schema for the resource class
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type RDSInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	SpecTemplate RDSInstanceClassSpecTemplate `json:"specTemplate,omitempty"`
}

// GetReclaimPolicy of this RDSInstanceClass.
func (i *RDSInstanceClass) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return i.SpecTemplate.ReclaimPolicy
}

// SetReclaimPolicy of this RDSInstanceClass.
func (i *RDSInstanceClass) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	i.SpecTemplate.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// RDSInstanceClassList contains a list of cloud memorystore resource classes.
type RDSInstanceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstanceClass `json:"items"`
}

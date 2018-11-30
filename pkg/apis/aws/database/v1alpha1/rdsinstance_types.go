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
	"strconv"
	"strings"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
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

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference      `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference      `json:"classRef,omitempty"`
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference  `json:"providerRef"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

type RDSInstanceState string

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
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	State        string `json:"state,omitempty"`
	Message      string `json:"message,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`   // the external ID to identify this resource in the cloud provider
	InstanceName string `json:"instanceName,omitempty"` // the generated DB Instance name
	Endpoint     string `json:"endpoint,omitempty"`     // rds instance endpoint
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstance is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.aws
type RDSInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSInstanceSpec   `json:"spec,omitempty"`
	Status RDSInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceList contains a list of RDSInstance
type RDSInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstance `json:"items"`
}

// NewRDSInstanceSpec from properties map
func NewRDSInstanceSpec(properties map[string]string) *RDSInstanceSpec {
	spec := &RDSInstanceSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}

	val, ok := properties["masterUsername"]
	if ok {
		spec.MasterUsername = val
	}

	val, ok = properties["engineVersion"]
	if ok {
		spec.EngineVersion = val
	}

	val, ok = properties["class"]
	if ok {
		spec.Class = val
	}

	val, ok = properties["size"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.Size = int64(size)
		}
	}

	val, ok = properties["securityGroups"]
	if ok {
		spec.SecurityGroups = append(spec.SecurityGroups, strings.Split(val, ",")...)
	}

	val, ok = properties["subnetGroupName"]
	if ok {
		spec.SubnetGroupName = val
	}

	return spec
}

// ConnectionSecretName returns a secret name from the reference
func (r *RDSInstance) ConnectionSecretName() string {
	if r.Spec.ConnectionSecretRef == nil {
		r.Spec.ConnectionSecretRef = &corev1.LocalObjectReference{
			Name: r.Name,
		}
	} else if r.Spec.ConnectionSecretRef.Name == "" {
		r.Spec.ConnectionSecretRef.Name = r.Name
	}

	return r.Spec.ConnectionSecretRef.Name
}

// Endpoint returns rds instance endpoint value saved in the status (could be empty)
func (r *RDSInstance) Endpoint() string {
	return r.Status.Endpoint
}

// SetEndpoint sets status endpoint field
func (r *RDSInstance) SetEndpoint(s string) {
	r.Status.Endpoint = s
}

// ObjectReference to this RDSInstance
func (r *RDSInstance) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(r.ObjectMeta, util.IfEmptyString(r.APIVersion, APIVersion), util.IfEmptyString(r.Kind, RDSInstanceKind))
}

// OwnerReference to use this instance as an owner
func (r *RDSInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(r.ObjectReference())
}

// State returns rds instance state value saved in the status (could be empty)
func (r *RDSInstance) State() string {
	return r.Status.State
}

// SetState sets status state field
func (r *RDSInstance) SetState(s string) {
	r.Status.State = s
}

// IsAvailable for usage/binding
func (r *RDSInstance) IsAvailable() bool {
	return r.State() == string(RDSInstanceStateAvailable)
}

// IsBound
func (r *RDSInstance) IsBound() bool {
	return r.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound
func (r *RDSInstance) SetBound(state bool) {
	if state {
		r.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		r.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}

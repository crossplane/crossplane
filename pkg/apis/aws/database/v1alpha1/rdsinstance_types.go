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
	"strconv"
	"strings"

	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
	MasterUsername string   `json:"masterUsername"`
	Engine         string   `json:"engine"`
	EngineVersion  string   `json:"engineVersion,omitempty"`
	Class          string   `json:"class"`                    // like "db.t2.micro"
	Size           int64    `json:"size"`                     // size in gb
	SecurityGroups []string `json:"securityGroups,omitempty"` // VPC Security groups

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference      `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference      `json:"classRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference  `json:"providerRef"`
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

type RDSInstanceState int

const (
	// The instance is healthy and available
	RDSInstanceStateAvailable RDSInstanceState = iota
	// The instance is being created. The instance is inaccessible while it is being created.
	RDSInstanceStateCreating
	// The instance is being deleted.
	RDSInstanceStateDeleting
	// The instance has failed and Amazon RDS can't recover it. Perform a point-in-time restore to the latest restorable time of the instance to recover the data.
	RDSInstanceStateFailed
)

func (s RDSInstanceState) String() string {
	return [...]string{"available", "creating", "deleting", "failed"}[s]
}

// ConditionType based on DBInstance status
func ConditionType(status string) corev1alpha1.ConditionType {
	switch status {
	case RDSInstanceStateAvailable.String():
		return corev1alpha1.Running
	case RDSInstanceStateCreating.String():
		return corev1alpha1.Creating
	case RDSInstanceStateDeleting.String():
		return corev1alpha1.Deleting
	case RDSInstanceStateFailed.String():
		return corev1alpha1.Failed
	default:
		return corev1alpha1.Pending
	}
}

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
	if r.Kind == "" {
		r.Kind = RDSInstanceKind
	}
	if r.APIVersion == "" {
		r.APIVersion = APIVersion
	}
	return &corev1.ObjectReference{
		APIVersion:      r.APIVersion,
		Kind:            r.Kind,
		Name:            r.Name,
		Namespace:       r.Namespace,
		ResourceVersion: r.ResourceVersion,
		UID:             r.UID,
	}
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
	return r.State() == RDSInstanceStateAvailable.String()
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

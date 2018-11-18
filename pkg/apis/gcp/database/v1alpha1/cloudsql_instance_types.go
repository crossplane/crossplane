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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// StateRunnable represents a CloudSQL instance in a running, available, and ready state
	StateRunnable = "RUNNABLE"

	// StatePendingCreate represents a CloudSQL instance that is in the process of being created
	StatePendingCreate = "PENDING_CREATE"

	// StateFailed  represents a CloudSQL instance has failed in some way
	StateFailed = "FAILED"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CloudsqlInstanceSpec defines the desired state of CloudsqlInstance
type CloudsqlInstanceSpec struct {
	Tier            string `json:"tier"`
	Region          string `json:"region"`
	DatabaseVersion string `json:"databaseVersion"`
	StorageType     string `json:"storageType"`

	// Kubernetes object references
	ClaimRef            *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef            *v1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// CloudsqlInstanceStatus defines the observed state of CloudsqlInstance
type CloudsqlInstanceStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// the external ID to identify this resource in the cloud provider
	ProviderID string `json:"providerID,omitempty"`

	// Endpoint of the Cloud SQL instance used in connection strings.
	Endpoint string `json:"endpoint,omitempty"`

	// Name of the Cloud SQL instance. This does not include the project ID.
	InstanceName string `json:"instanceName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudsqlInstance is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.gcp
type CloudsqlInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudsqlInstanceSpec   `json:"spec,omitempty"`
	Status CloudsqlInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudsqlInstanceList contains a list of CloudsqlInstance
type CloudsqlInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudsqlInstance `json:"items"`
}

// NewCloudSQLInstanceSpec creates a new CloudSQLInstanceSpec based on the given properties map
func NewCloudSQLInstanceSpec(properties map[string]string) *CloudsqlInstanceSpec {
	spec := &CloudsqlInstanceSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}

	val, ok := properties["tier"]
	if ok {
		spec.Tier = val
	}

	val, ok = properties["region"]
	if ok {
		spec.Region = val
	}

	val, ok = properties["databaseVersion"]
	if ok {
		spec.DatabaseVersion = val
	}

	val, ok = properties["storageType"]
	if ok {
		spec.StorageType = val
	}

	return spec
}

// ConnectionSecretName returns a secret name from the reference
func (c *CloudsqlInstance) ConnectionSecretName() string {
	if c.Spec.ConnectionSecretRef.Name == "" {
		// the user hasn't specified the name of the secret they want the connection information
		// stored in, generate one now
		c.Spec.ConnectionSecretRef.Name = c.Name
	}

	return c.Spec.ConnectionSecretRef.Name
}

// Endpoint returns the CloudSQL instance endpoint for connection
func (c *CloudsqlInstance) Endpoint() string {
	return c.Status.Endpoint
}

// ObjectReference to this CloudSQL instance instance
func (c *CloudsqlInstance) ObjectReference() *v1.ObjectReference {
	return util.ObjectReference(c.ObjectMeta, util.IfEmptyString(c.APIVersion, APIVersion), util.IfEmptyString(c.Kind, CloudsqlInstanceKind))
}

// OwnerReference to use this instance as an owner
func (c *CloudsqlInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(c.ObjectReference())
}

// IsAvailable for usage/binding
func (c *CloudsqlInstance) IsAvailable() bool {
	return c.Status.State == StateRunnable
}

// IsBound determines if the resource is in a bound binding state
func (c *CloudsqlInstance) IsBound() bool {
	return c.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound sets the binding state of this resource
func (c *CloudsqlInstance) SetBound(state bool) {
	if state {
		c.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		c.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}

// ValidVersionValues returns the valid set of engine version values.
func ValidVersionValues() map[string]string {
	return map[string]string{
		"5.5": "MYSQL_5_5",
		"5.6": "MYSQL_5_6",
		"5.7": "MYSQL_5_7",
	}
}

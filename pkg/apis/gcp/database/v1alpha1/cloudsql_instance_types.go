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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// CloudSQL instance states
const (
	// StateRunnable represents a CloudSQL instance in a running, available, and ready state
	StateRunnable = "RUNNABLE"

	// StatePendingCreate represents a CloudSQL instance that is in the process of being created
	StatePendingCreate = "PENDING_CREATE"

	// StateFailed  represents a CloudSQL instance has failed in some way
	StateFailed = "FAILED"
)

// CloudSQL version prefixes.
const (
	MysqlDBVersionPrefix      = "MYSQL"
	PostgresqlDBVersionPrefix = "POSTGRES"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CloudsqlInstanceSpec defines the desired state of CloudsqlInstance
type CloudsqlInstanceSpec struct {
	Region      string `json:"region"`
	StorageType string `json:"storageType"`
	StorageGB   int64  `json:"storageGB"`

	// The database engine (MySQL or PostgreSQL) and its specific version to use, e.g., MYSQL_5_7 or POSTGRES_9_6.
	DatabaseVersion string `json:"databaseVersion"`

	// MySQL and PostgreSQL use different machine types.  MySQL only allows a predefined set of machine types,
	// while PostgreSQL can only use custom machine instance types and shared-core instance types. For the full
	// set of MySQL machine types, see https://cloud.google.com/sql/pricing#2nd-gen-instance-pricing. For more
	// information on custom machine types that can be used with PostgreSQL, see the examples on
	// https://cloud.google.com/sql/docs/postgres/create-instance?authuser=1#machine-types and the naming rules
	// on https://cloud.google.com/sql/docs/postgres/create-instance#create-2ndgen-curl.
	Tier string `json:"tier"`

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
	corev1alpha1.DeprecatedConditionedStatus
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
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.databaseVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
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

	val, ok = properties["storageGB"]
	if ok {
		if storageGB, err := strconv.Atoi(val); err == nil {
			spec.StorageGB = int64(storageGB)
		}
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
	return c.Status.IsBound()
}

// SetBound sets the binding state of this resource
func (c *CloudsqlInstance) SetBound(bound bool) {
	c.Status.SetBound(bound)
}

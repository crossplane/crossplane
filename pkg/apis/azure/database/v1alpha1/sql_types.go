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

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	// OperationCreateServer is the operation type for creating a new mysql server
	OperationCreateServer = "createServer"
	// OperationCreateFirewallRules is the operation type for creating a firewall rule
	OperationCreateFirewallRules = "createFirewallRules"
)

type SqlServer interface {
	corev1alpha1.Resource
	metav1.Object
	OwnerReference() metav1.OwnerReference
	GetSpec() *SQLServerSpec
	GetStatus() *SQLServerStatus
	SetStatus(*SQLServerStatus)
}

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlServer is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.azure
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type MysqlServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SQLServerSpec   `json:"spec,omitempty"`
	Status SQLServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlServerList contains a list of MysqlServer
type MysqlServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlServer `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlServer is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.azure
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type PostgresqlServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SQLServerSpec   `json:"spec,omitempty"`
	Status SQLServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlServerList contains a list of PostgresqlServer
type PostgresqlServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlServer `json:"items"`
}

// SQLServerSpec defines the desired state of SQLServer
type SQLServerSpec struct {
	ResourceGroupName string             `json:"resourceGroupName"`
	Location          string             `json:"location"`
	PricingTier       PricingTierSpec    `json:"pricingTier"`
	StorageProfile    StorageProfileSpec `json:"storageProfile"`
	AdminLoginName    string             `json:"adminLoginName"`
	Version           string             `json:"version"`
	SSLEnforced       bool               `json:"sslEnforced,omitempty"`

	// Kubernetes object references
	ClaimRef            *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef            *v1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// SQLServerStatus defines the observed state of SQLServer
type SQLServerStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// the external ID to identify this resource in the cloud provider
	ProviderID string `json:"providerID,omitempty"`

	// Endpoint of the MySQL Server instance used in connection strings
	Endpoint string `json:"endpoint,omitempty"`

	// RunningOperation stores any current long running operation for this instance across
	// reconciliation attempts.  This will be a serialized Azure MySQL Server API object that will
	// be used to check the status and completion of the operation during each reconciliation.
	// Once the operation has completed, this field will be cleared out.
	RunningOperation string `json:"runningOperation,omitempty"`

	// RunningOperationType is the type of the currently running operation
	RunningOperationType string `json:"runningOperationType,omitempty"`
}

// PricingTierSpec represents the performance and cost oriented properties of the server
type PricingTierSpec struct {
	Tier   string `json:"tier"`
	VCores int    `json:"vcores"`
	Family string `json:"family"`
}

// StorageProfileSpec represents storage related properties of the server
type StorageProfileSpec struct {
	StorageGB           int  `json:"storageGB"`
	BackupRetentionDays int  `json:"backupRetentionDays,omitempty"`
	GeoRedundantBackup  bool `json:"geoRedundantBackup,omitempty"`
}

// NewSQLServerSpec creates a new SQLServerSpec based on the given properties map
func NewSQLServerSpec(properties map[string]string) *SQLServerSpec {
	spec := &SQLServerSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}

	val, ok := properties["adminLoginName"]
	if ok {
		spec.AdminLoginName = val
	}

	val, ok = properties["resourceGroupName"]
	if ok {
		spec.ResourceGroupName = val
	}

	val, ok = properties["location"]
	if ok {
		spec.Location = val
	}

	val, ok = properties["version"]
	if ok {
		spec.Version = val
	}

	val, ok = properties["sslEnforced"]
	if ok {
		if sslEnforced, err := strconv.ParseBool(val); err == nil {
			spec.SSLEnforced = sslEnforced
		}
	}

	val, ok = properties["tier"]
	if ok {
		spec.PricingTier.Tier = val
	}

	val, ok = properties["vcores"]
	if ok {
		if vcores, err := strconv.Atoi(val); err == nil {
			spec.PricingTier.VCores = vcores
		}
	}

	val, ok = properties["family"]
	if ok {
		spec.PricingTier.Family = val
	}

	val, ok = properties["storageGB"]
	if ok {
		if storageGB, err := strconv.Atoi(val); err == nil {
			spec.StorageProfile.StorageGB = storageGB
		}
	}

	val, ok = properties["backupRetentionDays"]
	if ok {
		if backupRetentionDays, err := strconv.Atoi(val); err == nil {
			spec.StorageProfile.BackupRetentionDays = backupRetentionDays
		}
	}

	val, ok = properties["geoRedundantBackup"]
	if ok {
		if geoRedundantBackup, err := strconv.ParseBool(val); err == nil {
			spec.StorageProfile.GeoRedundantBackup = geoRedundantBackup
		}
	}

	return spec
}

//---------------------------------------------------------------------------------------------------------------------
// MysqlServer

func (m *MysqlServer) GetSpec() *SQLServerSpec {
	return &m.Spec
}

func (m *MysqlServer) GetStatus() *SQLServerStatus {
	return &m.Status
}

func (m *MysqlServer) SetStatus(status *SQLServerStatus) {
	m.Status = *status
}

// ConnectionSecretName returns a secret name from the reference
func (m *MysqlServer) ConnectionSecretName() string {
	if m.Spec.ConnectionSecretRef.Name == "" {
		// the user hasn't specified the name of the secret they want the connection information
		// stored in, generate one now
		m.Spec.ConnectionSecretRef.Name = m.Name
	}

	return m.Spec.ConnectionSecretRef.Name
}

// ObjectReference to this MySQL Server instance
func (m *MysqlServer) ObjectReference() *v1.ObjectReference {
	return util.ObjectReference(m.ObjectMeta, util.IfEmptyString(m.APIVersion, APIVersion), util.IfEmptyString(m.Kind, MysqlServerKind))
}

// OwnerReference to use this instance as an owner
func (m *MysqlServer) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(m.ObjectReference())
}

// IsAvailable for usage/binding
func (m *MysqlServer) IsAvailable() bool {
	return m.Status.State == string(mysql.ServerStateReady)
}

// IsBound determines if the resource is in a bound binding state
func (m *MysqlServer) IsBound() bool {
	return m.Status.IsBound()
}

// SetBound sets the binding state of this resource
func (m *MysqlServer) SetBound(bound bool) {
	m.Status.SetBound(bound)
}

//---------------------------------------------------------------------------------------------------------------------
// PostgresqlServer

func (p *PostgresqlServer) GetSpec() *SQLServerSpec {
	return &p.Spec
}

func (p *PostgresqlServer) GetStatus() *SQLServerStatus {
	return &p.Status
}

func (p *PostgresqlServer) SetStatus(status *SQLServerStatus) {
	p.Status = *status
}

// ConnectionSecretName returns a secret name from the reference
func (p *PostgresqlServer) ConnectionSecretName() string {
	if p.Spec.ConnectionSecretRef.Name == "" {
		// the user hasn't specified the name of the secret they want the connection information
		// stored in, generate one now
		p.Spec.ConnectionSecretRef.Name = p.Name
	}

	return p.Spec.ConnectionSecretRef.Name
}

// ObjectReference to this PostgreSQL Server instance
func (p *PostgresqlServer) ObjectReference() *v1.ObjectReference {
	return util.ObjectReference(p.ObjectMeta, util.IfEmptyString(p.APIVersion, APIVersion), util.IfEmptyString(p.Kind, PostgresqlServerKind))
}

// OwnerReference to use this instance as an owner
func (p *PostgresqlServer) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(p.ObjectReference())
}

// IsAvailable for usage/binding
func (p *PostgresqlServer) IsAvailable() bool {
	return p.Status.State == string(postgresql.ServerStateReady)
}

// IsBound determines if the resource is in a bound binding state
func (p *PostgresqlServer) IsBound() bool {
	return p.Status.IsBound()
}

// SetBound sets the binding state of this resource
func (p *PostgresqlServer) SetBound(bound bool) {
	p.Status.SetBound(bound)
}

// ValidMySQLVersionValues returns the valid set of engine version values.
func ValidMySQLVersionValues() []string {
	return []string{"5.6", "5.7"}
}

// ValidPostgreSQLVersionValues returns the valid set of engine version values.
func ValidPostgreSQLVersionValues() []string {
	return []string{"9.5", "9.6", "10", "10.0", "10.2"}
}

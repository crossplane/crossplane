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

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlServer is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.azure
type MysqlServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlServerSpec   `json:"spec,omitempty"`
	Status MysqlServerStatus `json:"status,omitempty"`
}

// MysqlServerSpec defines the desired state of MysqlServer
type MysqlServerSpec struct {
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

// MysqlServerStatus defines the observed state of MysqlServer
type MysqlServerStatus struct {
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlServerList contains a list of MysqlServer
type MysqlServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlServer `json:"items"`
}

// NewMySQLServerSpec creates a new MySQLServerSpec based on the given properties map
func NewMySQLServerSpec(properties map[string]string) *MysqlServerSpec {
	spec := &MysqlServerSpec{
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

// ConnectionSecretName returns a secret name from the reference
func (m *MysqlServer) ConnectionSecretName() string {
	if m.Spec.ConnectionSecretRef.Name == "" {
		// the user hasn't specified the name of the secret they want the connection information
		// stored in, generate one now
		m.Spec.ConnectionSecretRef.Name = m.Name
	}

	return m.Spec.ConnectionSecretRef.Name
}

// Endpoint returns the MySQL Server endpoint for connection
func (m *MysqlServer) Endpoint() string {
	return m.Status.Endpoint
}

// ObjectReference to this MySQL Server instance
func (m *MysqlServer) ObjectReference() *v1.ObjectReference {
	if m.Kind == "" {
		m.Kind = MysqlServerKind
	}
	if m.APIVersion == "" {
		m.APIVersion = APIVersion
	}
	return &v1.ObjectReference{
		APIVersion:      m.APIVersion,
		Kind:            m.Kind,
		Name:            m.Name,
		Namespace:       m.Namespace,
		ResourceVersion: m.ResourceVersion,
		UID:             m.UID,
	}
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
	return m.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound sets the binding state of this resource
func (m *MysqlServer) SetBound(state bool) {
	if state {
		m.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		m.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}

// ValidVersionValues returns the valid set of engine version values.
func ValidVersionValues() []string {
	return []string{"5.6", "5.7"}
}

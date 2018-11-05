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
	"github.com/upbound/conductor/pkg/apis/core/v1alpha1"
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
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	ResourceGroupName   string                  `json:"resourceGroupName"`
	Location            string                  `json:"location"`
	PricingTier         PricingTierSpec         `json:"pricingTier"`
	StorageProfile      StorageProfileSpec      `json:"storageProfile"`
	AdminLoginName      string                  `json:"adminLoginName"`
	Version             string                  `json:"version"`
	SSLEnforced         bool                    `json:"sslEnforced,omitempty"`
}

// MysqlServerStatus defines the observed state of MysqlServer
type MysqlServerStatus struct {
	v1alpha1.ConditionedStatus
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

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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
	MasterUsername        string               `json:"masterUsername"`
	MasterPassword        v1.SecretKeySelector `json:"masterPasswordSecretRef"`
	Engine                string               `json:"engine"` // "postgres"
	Class                 string               `json:"class"`  // like "db.t2.micro"
	Size                  int64                `json:"size"`   // size in gb
	MultiAZ               bool                 `json:"multiaz,omitempty"`
	PubliclyAccessible    bool                 `json:"publicaccess,omitempty"`
	StorageEncrypted      bool                 `json:"encrypted,omitempty"`
	StorageType           string               `json:"storagetype,omitempty"`
	Iops                  int64                `json:"iops,omitempty"`
	BackupRetentionPeriod int64                `json:"backupretentionperiod,omitempty"` // between 0 and 35, zero means disable
}

// RDSInstanceStatus defines the observed state of RDSInstance
type RDSInstanceStatus struct {
	State      string `json:"state,omitempty"`
	Message    string `json:"message,omitempty"`
	ProviderID string `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstance is the Schema for the instances API
// +k8s:openapi-gen=true
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

func init() {
	SchemeBuilder.Register(&RDSInstance{}, &RDSInstanceList{})
}

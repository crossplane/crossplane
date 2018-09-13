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

// RDSSpec defines the desired state of RDS
type RDSSpec struct {
	// Important: Run "make" to regenerate code after modifying this file
	Username              string               `json:"username"`
	Password              v1.SecretKeySelector `json:"password"`
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

// RDSStatus defines the observed state of RDS
type RDSStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	State      string `json:"state,omitempty"`
	Message    string `json:"message,omitempty"`
	ProviderID string `json:"providerID,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDS is the Schema for the rds API
// +k8s:openapi-gen=true
type RDS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSSpec   `json:"spec,omitempty"`
	Status RDSStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSList contains a list of RDS
type RDSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RDS{}, &RDSList{})
}

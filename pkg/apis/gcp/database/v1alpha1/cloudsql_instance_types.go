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

// CloudsqlInstanceSpec defines the desired state of CloudsqlInstance
type CloudsqlInstanceSpec struct {
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	Tier                string                  `json:"tier"`
	Region              string                  `json:"region"`
	DatabaseVersion     string                  `json:"databaseVersion"`
	StorageType         string                  `json:"storageType"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef"`
}

// CloudsqlInstanceStatus defines the observed state of CloudsqlInstance
type CloudsqlInstanceStatus struct {
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// the external ID to identify this resource in the cloud provider
	ProviderID string `json:"providerID,omitempty"`

	// Connection name of the Cloud SQL instance used in connection strings.
	ConnectionName string `json:"connectionName,omitempty"`

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

func init() {
	SchemeBuilder.Register(&CloudsqlInstance{}, &CloudsqlInstanceList{})
}

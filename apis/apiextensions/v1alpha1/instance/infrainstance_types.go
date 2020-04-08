/*
Copyright 2020 The Crossplane Authors.

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
// +kubebuilder:skip

package instance

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// InfraInstanceSpec specifies the desired state of a InfraInstance.
type InfraInstanceSpec struct {
	ResourceInstanceCommonSpec `json:",inline"`

	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// An InfraInstance is the internal representation of the resource generated
// via InfrastructureDefinition. It is only used for operations in the controller,
// it's not intended to be stored in the api-server.
type InfraInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfraInstanceSpec                   `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// InfraInstanceList contains a list of InfraInstances.
type InfraInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfraInstance `json:"items"`
}

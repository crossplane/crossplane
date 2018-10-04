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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&RDSInstance{}, &RDSInstanceList{})
}

// RDSInstanceConditionType type for possible conditions the provider could be in.
type RDSInstanceConditionType string

const (
	// Pending means that the instance create request has been received and waiting to be fulfilled
	Pending RDSInstanceConditionType = "Pending"
	// Creating means that the DB instance create request has been processed and DB Instance is being created
	Creating RDSInstanceConditionType = "Creating"
	// Deleting means that the instance is being deleted.
	Deleting RDSInstanceConditionType = "Deleting"
	// Failed means that the instance creation has failed.
	Failed RDSInstanceConditionType = "Failed"
	// Running means that the instance creation has been successful.
	Running RDSInstanceConditionType = "Running"
)

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
	MasterUsername string   `json:"masterUsername"`
	Engine         string   `json:"engine"`                   // "postgres"
	Class          string   `json:"class"`                    // like "db.t2.micro"
	Size           int64    `json:"size"`                     // size in gb
	SecurityGroups []string `json:"securityGroups,omitempty"` // VPC Security groups

	ProviderRef         corev1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef"`
}

// RDSInstanceStatus defines the observed state of RDSInstance
type RDSInstanceStatus struct {
	State        string `json:"state,omitempty"`
	Message      string `json:"message,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`   // the external ID to identify this resource in the cloud provider
	InstanceName string `json:"instanceName,omitempty"` // the generated DB Instance name

	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []RDSInstanceCondition
}

// GetCondition returns a provider condition with the provided type if it exists.
func (s *RDSInstanceStatus) GetCondition(conditionType RDSInstanceConditionType) *RDSInstanceCondition {
	for _, c := range s.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the credentials controller status.
func (s *RDSInstanceStatus) SetCondition(condition RDSInstanceCondition) {
	current := s.GetCondition(condition.Type)
	if current != nil && current.Status == condition.Status && current.Reason == condition.Reason {
		return
	}
	newConditions := FilterOutCondition(s.Conditions, condition.Type)
	s.Conditions = append(newConditions, condition)
}

// UnsetCondition set condition status to false with the given type - if found.
func (s *RDSInstanceStatus) UnsetCondition(conditionType RDSInstanceConditionType) {
	current := s.GetCondition(conditionType)
	if current != nil && current.Status == corev1.ConditionTrue {
		current.Status = corev1.ConditionFalse
		s.SetCondition(*current)
	}
}

// UnsetAllConditions set conditions status to false on all conditions
func (s *RDSInstanceStatus) UnsetAllConditions() {
	var newConditions []RDSInstanceCondition
	for _, c := range s.Conditions {
		c.Status = corev1.ConditionFalse
		newConditions = append(newConditions, c)
	}
	s.Conditions = newConditions
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func (s *RDSInstanceStatus) RemoveCondition(condType RDSInstanceConditionType) {
	s.Conditions = FilterOutCondition(s.Conditions, condType)
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

// RDSInstanceCondition contains details for the current condition of this pod.
type RDSInstanceCondition struct {
	Type               RDSInstanceConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// NewCondition creates a new RDS instance condition.
func NewCondition(condType RDSInstanceConditionType, reason, msg string) *RDSInstanceCondition {
	return &RDSInstanceCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// FilterOutCondition returns a new slice of credentials controller conditions without conditions with the provided type.
func FilterOutCondition(conditions []RDSInstanceCondition, condType RDSInstanceConditionType) []RDSInstanceCondition {
	var newConditions []RDSInstanceCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

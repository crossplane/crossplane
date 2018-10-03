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

// ConditionType type for possible conditions the resource could be in.
type ConditionType string

const (
	// Pending means that the instance create request has been received and waiting to be fulfilled
	Pending ConditionType = "Pending"
	// Creating means that the DB instance create request has been processed and DB Instance is being created
	Creating ConditionType = "Creating"
	// Deleting means that the instance is being deleted.
	Deleting ConditionType = "Deleting"
	// Failed means that the instance creation has failed.
	Failed ConditionType = "Failed"
	// Running means that the instance creation has been successful.
	Running ConditionType = "Running"
)

// Condition contains details for the current condition of this pod.
type Condition struct {
	Type               ConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// ConditionedStatus defines the observed state of RDSInstance
type ConditionedStatus struct {
	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []Condition
}

// GetCondition returns a provider condition with the provided type if it exists.
func (in *ConditionedStatus) GetCondition(conditionType ConditionType) *Condition {
	for _, c := range in.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the credentials controller status.
func (in *ConditionedStatus) SetCondition(condition Condition) {
	current := in.GetCondition(condition.Type)
	if current != nil && current.Status == condition.Status && current.Reason == condition.Reason {
		return
	}
	newConditions := FilterOutCondition(in.Conditions, condition.Type)
	in.Conditions = append(newConditions, condition)
}

// UnsetCondition set condition status to false with the given type - if found.
func (in *ConditionedStatus) UnsetCondition(conditionType ConditionType) {
	current := in.GetCondition(conditionType)
	if current != nil && current.Status == corev1.ConditionTrue {
		current.Status = corev1.ConditionFalse
		in.SetCondition(*current)
	}
}

// UnsetAllConditions set conditions status to false on all conditions
func (in *ConditionedStatus) UnsetAllConditions() {
	var newConditions []Condition
	for _, c := range in.Conditions {
		c.Status = corev1.ConditionFalse
		newConditions = append(newConditions, c)
	}
	in.Conditions = newConditions
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func (in *ConditionedStatus) RemoveCondition(condType ConditionType) {
	in.Conditions = FilterOutCondition(in.Conditions, condType)
}

// NewCondition creates a new RDS instance condition.
func NewCondition(condType ConditionType, reason, msg string) *Condition {
	return &Condition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// FilterOutProviderCondition returns a new slice of credentials controller conditions without conditions with the provided type.
func FilterOutCondition(conditions []Condition, condType ConditionType) []Condition {
	var newConditions []Condition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

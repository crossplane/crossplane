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

// ProviderConditionType type for possible conditions the provider could be in.
type ProviderConditionType string

const (
	// Valid means that provider's credentials has been processed and validated
	Valid ProviderConditionType = "Valid"
	// Invalid means that provider's credentials has been processed and deemed invalid
	Invalid ProviderConditionType = "Invalid"
)

// ProviderCondition contains details for the current condition of this pod.
type ProviderCondition struct {
	Type               ProviderConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []ProviderCondition
}

// NewCondition creates a provider condition.
func NewProviderCondition(condType ProviderConditionType, status corev1.ConditionStatus, reason, msg string) *ProviderCondition {
	return &ProviderCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// GetCondition returns a provider condition with the provided type if it exists.
func (in *ProviderStatus) GetCondition(conditionType ProviderConditionType) *ProviderCondition {
	for _, c := range in.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the credentials controller status.
func (in *ProviderStatus) SetCondition(condition ProviderCondition) {
	current := in.GetCondition(condition.Type)
	if current != nil && current.Status == condition.Status && current.Reason == condition.Reason {
		return
	}
	newConditions := FilterOutProviderCondition(in.Conditions, condition.Type)
	in.Conditions = append(newConditions, condition)
}

// SetInvalid condition and unset valid condition
func (in *ProviderStatus) SetInvalid(reason, msg string) {
	in.SetCondition(*NewProviderCondition(Invalid, corev1.ConditionTrue, reason, msg))

	if valid := in.GetCondition(Valid); valid != nil {
		in.SetCondition(*NewProviderCondition(Valid, corev1.ConditionFalse, "", valid.Message))
	}
}

// SetValid condition and unset invalid condition
func (in *ProviderStatus) SetValid(msg string) {
	in.SetCondition(*NewProviderCondition(Valid, corev1.ConditionTrue, "", msg))

	if invalid := in.GetCondition(Invalid); invalid != nil {
		in.SetCondition(*NewProviderCondition(Invalid, corev1.ConditionFalse, invalid.Reason, invalid.Message))
	}
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func (in *ProviderStatus) RemoveCondition(condType ProviderConditionType) {
	in.Conditions = FilterOutProviderCondition(in.Conditions, condType)
}

// FilterOutProviderCondition returns a new slice of credentials controller conditions without conditions with the provided type.
func FilterOutProviderCondition(conditions []ProviderCondition, condType ProviderConditionType) []ProviderCondition {
	var newConditions []ProviderCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

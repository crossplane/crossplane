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

package provider

import (
	conductorcorev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCondition creates a new replication controller condition.
func NewCondition(condType conductorcorev1alpha1.ProviderConditionType, status corev1.ConditionStatus, reason, msg string) *conductorcorev1alpha1.ProviderCondition {
	return &conductorcorev1alpha1.ProviderCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// GetCondition returns a credentials controller condition with the provided type if it exists.
func GetCondition(status conductorcorev1alpha1.ProviderStatus, conditionType conductorcorev1alpha1.ProviderConditionType) *conductorcorev1alpha1.ProviderCondition {
	for _, c := range status.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the credentials controller status.
func SetCondition(status *conductorcorev1alpha1.ProviderStatus, condition conductorcorev1alpha1.ProviderCondition) {
	current := GetCondition(*status, condition.Type)
	if current != nil && current.Status == condition.Status && current.Reason == condition.Reason {
		return
	}
	newConditions := FilterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// SetInvalid condition and unset valid condition
func SetInvalid(status *conductorcorev1alpha1.ProviderStatus, reason, msg string) {
	SetCondition(status, *NewCondition(conductorcorev1alpha1.Invalid, corev1.ConditionTrue, reason, msg))

	if valid := GetCondition(*status, conductorcorev1alpha1.Valid); valid != nil {
		SetCondition(status, *NewCondition(conductorcorev1alpha1.Valid, corev1.ConditionFalse, "", valid.Message))
	}
}

// SetValid condition and unset invalid condition
func SetValid(status *conductorcorev1alpha1.ProviderStatus, msg string) {
	SetCondition(status, *NewCondition(conductorcorev1alpha1.Valid, corev1.ConditionTrue, "", msg))

	if invalid := GetCondition(*status, conductorcorev1alpha1.Invalid); invalid != nil {
		SetCondition(status, *NewCondition(conductorcorev1alpha1.Invalid, corev1.ConditionFalse, invalid.Reason, invalid.Message))
	}
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func RemoveCondition(status *conductorcorev1alpha1.ProviderStatus, condType conductorcorev1alpha1.ProviderConditionType) {
	status.Conditions = FilterOutCondition(status.Conditions, condType)
}

// FilterOutCondition returns a new slice of credentials controller conditions without conditions with the provided type.
func FilterOutCondition(conditions []conductorcorev1alpha1.ProviderCondition, condType conductorcorev1alpha1.ProviderConditionType) []conductorcorev1alpha1.ProviderCondition {
	var newConditions []conductorcorev1alpha1.ProviderCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

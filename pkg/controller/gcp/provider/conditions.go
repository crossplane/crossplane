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
	"github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newCredentialsCondition creates a new replication controller condition.
func newCondition(condType v1alpha1.ProviderConditionType, status corev1.ConditionStatus, reason, msg string) *v1alpha1.ProviderCondition {
	return &v1alpha1.ProviderCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// getCondition returns a credentials controller condition with the provided type if it exists.
func getCondition(status v1alpha1.ProviderStatus, conditionType v1alpha1.ProviderConditionType) *v1alpha1.ProviderCondition {
	for _, c := range status.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// setCondition adds/replaces the given condition in the credentials controller status.
func setCondition(status *v1alpha1.ProviderStatus, condition v1alpha1.ProviderCondition) {
	current := getCondition(*status, condition.Type)
	if current != nil && current.Status == condition.Status && current.Reason == condition.Reason {
		return
	}
	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// setInvalid condition and unset valid condition
func setInvalid(status *v1alpha1.ProviderStatus, reason, msg string) {
	setCondition(status, *newCondition(v1alpha1.Invalid, corev1.ConditionTrue, reason, msg))

	if valid := getCondition(*status, v1alpha1.Valid); valid != nil {
		setCondition(status, *newCondition(v1alpha1.Valid, corev1.ConditionFalse, "", valid.Message))
	}
}

// setValid condition and unset invalid condition
func setValid(status *v1alpha1.ProviderStatus, msg string) {
	setCondition(status, *newCondition(v1alpha1.Valid, corev1.ConditionTrue, "", msg))

	if invalid := getCondition(*status, v1alpha1.Invalid); invalid != nil {
		setCondition(status, *newCondition(v1alpha1.Invalid, corev1.ConditionFalse, invalid.Reason, invalid.Message))
	}
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func removeCondition(status *v1alpha1.ProviderStatus, condType v1alpha1.ProviderConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of credentials controller conditions without conditions with the provided type.
func filterOutCondition(conditions []v1alpha1.ProviderCondition, condType v1alpha1.ProviderConditionType) []v1alpha1.ProviderCondition {
	var newConditions []v1alpha1.ProviderCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

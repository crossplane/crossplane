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

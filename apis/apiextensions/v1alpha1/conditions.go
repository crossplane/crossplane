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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Condition types.
const (
	// TypeCurrent indicates whether a CompositionRevision is 'current' -
	// i.e. whether it matches the current state of its Composition.
	TypeCurrent xpv1.ConditionType = "Current"
)

// Reasons a package is or is not current.
const (
	ReasonCompositionSpecMatches xpv1.ConditionReason = "CompositionSpecMatches"
	ReasonCompositionSpecDiffers xpv1.ConditionReason = "CompositionSpecDiffers"
)

// CompositionSpecMatches indicates that a revision is current because its spec
// matches the Composition's.
func CompositionSpecMatches() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeCurrent,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonCompositionSpecMatches,
	}
}

// CompositionSpecDiffers indicates that a revision is current because its spec
// differs from the Composition's.
func CompositionSpecDiffers() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeCurrent,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonCompositionSpecDiffers,
	}
}

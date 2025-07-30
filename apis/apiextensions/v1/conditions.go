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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// Condition types.
const (
	// A TypeEstablished XRD has created the CRD for its composite resource and
	// started a controller to reconcile instances of said resource.
	TypeEstablished xpv1.ConditionType = "Established"

	// A TypeOffered XRD has created the CRD for its composite resource claim
	// and started a controller to reconcile instances of said claim.
	TypeOffered xpv1.ConditionType = "Offered"

	// A TypeValidPipeline CompositionRevision has a valid function
	// pipeline.
	TypeValidPipeline xpv1.ConditionType = "ValidPipeline"
)

// Reasons a resource is or is not established or offered.
const (
	ReasonWatchingComposite xpv1.ConditionReason = "WatchingCompositeResource"
	ReasonWatchingClaim     xpv1.ConditionReason = "WatchingCompositeResourceClaim"

	ReasonTerminatingComposite xpv1.ConditionReason = "TerminatingCompositeResource"
	ReasonTerminatingClaim     xpv1.ConditionReason = "TerminatingCompositeResourceClaim"

	ReasonValidPipeline       xpv1.ConditionReason = "ValidPipeline"
	ReasonMissingCapabilities xpv1.ConditionReason = "MissingCapabilities"
)

// WatchingComposite indicates that Crossplane has defined and is watching for a
// new kind of composite resource.
func WatchingComposite() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingComposite,
	}
}

// TerminatingComposite indicates that Crossplane is terminating the controller
// for and removing the definition of a composite resource.
func TerminatingComposite() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingComposite,
	}
}

// WatchingClaim indicates that Crossplane has defined and is watching for a
// new kind of composite resource claim.
func WatchingClaim() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingClaim,
	}
}

// TerminatingClaim indicates that Crossplane is terminating the controller and
// removing the definition of a composite resource claim.
func TerminatingClaim() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingClaim,
	}
}

// ValidPipeline indicates that all functions in the CompositionRevision's
// pipeline are valid.
func ValidPipeline() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidPipeline,
	}
}

// MissingCapabilities indicates that one or more functions in the CompositionRevision's
// pipeline are missing required capabilities.
func MissingCapabilities(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonMissingCapabilities,
		Message:            message,
	}
}

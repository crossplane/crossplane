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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// Condition types.
const (
	// A TypeEstablished XRD has created the CRD for its composite resource and
	// started a controller to reconcile instances of said resource.
	TypeEstablished xpv2.ConditionType = "Established"

	// A TypeOffered XRD has created the CRD for its composite resource claim
	// and started a controller to reconcile instances of said claim.
	TypeOffered xpv2.ConditionType = "Offered"

	// A TypeValidPipeline CompositionRevision has a valid function
	// pipeline.
	TypeValidPipeline xpv2.ConditionType = "ValidPipeline"

	// A TypeResponsive indicates whether the resource is responsive to changes.
	TypeResponsive xpv2.ConditionType = "Responsive"
)

// Reasons a resource is or is not established or offered.
const (
	ReasonWatchingComposite xpv2.ConditionReason = "WatchingCompositeResource"
	ReasonWatchingClaim     xpv2.ConditionReason = "WatchingCompositeResourceClaim"

	ReasonTerminatingComposite xpv2.ConditionReason = "TerminatingCompositeResource"
	ReasonTerminatingClaim     xpv2.ConditionReason = "TerminatingCompositeResourceClaim"

	ReasonValidPipeline       xpv2.ConditionReason = "ValidPipeline"
	ReasonMissingCapabilities xpv2.ConditionReason = "MissingCapabilities"

	ReasonWatchCircuitOpen   xpv2.ConditionReason = "WatchCircuitOpen"
	ReasonWatchCircuitClosed xpv2.ConditionReason = "WatchCircuitClosed"
)

// WatchingComposite indicates that Crossplane has defined and is watching for a
// new kind of composite resource.
func WatchingComposite() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingComposite,
	}
}

// TerminatingComposite indicates that Crossplane is terminating the controller
// for and removing the definition of a composite resource.
func TerminatingComposite() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingComposite,
	}
}

// WatchingClaim indicates that Crossplane has defined and is watching for a
// new kind of composite resource claim.
func WatchingClaim() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingClaim,
	}
}

// TerminatingClaim indicates that Crossplane is terminating the controller and
// removing the definition of a composite resource claim.
func TerminatingClaim() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingClaim,
	}
}

// ValidPipeline indicates that all functions in the CompositionRevision's
// pipeline are valid.
func ValidPipeline() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidPipeline,
	}
}

// MissingCapabilities indicates that one or more functions in the CompositionRevision's
// pipeline are missing required capabilities.
func MissingCapabilities(message string) xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonMissingCapabilities,
		Message:            message,
	}
}

// WatchCircuitOpen indicates the circuit breaker is open due to excessive watch events.
func WatchCircuitOpen(triggeredBy string) xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeResponsive,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchCircuitOpen,
		Message:            fmt.Sprintf("Too many watch events from %s. Allowing events periodically.", triggeredBy),
	}
}

// WatchCircuitClosed indicates the circuit breaker is closed (normal operation).
func WatchCircuitClosed() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeResponsive,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchCircuitClosed,
	}
}

// IsSystemConditionType returns true if the condition type is a system
// condition. This includes both crossplane-runtime system conditions and
// apiextensions-specific system conditions like the circuit breaker.
func IsSystemConditionType(t xpv2.ConditionType) bool {
	// First check crossplane-runtime system conditions
	if xpv2.IsSystemConditionType(t) {
		return true
	}

	// Then check Crossplane-specific system conditions
	return t == TypeResponsive
}

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Condition types.
const (
	// A TypeInstalled indicates whether a package has been installed.
	TypeInstalled xpv1.ConditionType = "Installed"

	// A TypeHealthy indicates whether a package is healthy.
	TypeHealthy xpv1.ConditionType = "Healthy"

	// A TypeRevisionHealthy indicates whether a package revision is healthy.
	TypeRevisionHealthy xpv1.ConditionType = "RevisionHealthy"

	// A TypeRuntimeHealthy indicates whether a package revision runtime is healthy.
	TypeRuntimeHealthy xpv1.ConditionType = "RuntimeHealthy"

	// A TypeVerified indicates whether a package's signature is verified.
	// It could be either successful or skipped to be marked as complete.
	TypeVerified xpv1.ConditionType = "Verified"
)

// Reasons a package is or is not installed.
const (
	ReasonAwaitingVerification xpv1.ConditionReason = "AwaitingSignatureVerification"
	ReasonUnpacking            xpv1.ConditionReason = "UnpackingPackage"
	ReasonInactive             xpv1.ConditionReason = "InactivePackageRevision"
	ReasonActive               xpv1.ConditionReason = "ActivePackageRevision"
	ReasonUnhealthy            xpv1.ConditionReason = "UnhealthyPackageRevision"
	ReasonHealthy              xpv1.ConditionReason = "HealthyPackageRevision"
	ReasonUnknownHealth        xpv1.ConditionReason = "UnknownPackageRevisionHealth"
)

// Reasons a package's signature is or is not verified.
const (
	// ReasonVerificationIncomplete indicates that signature verification is
	// not yet complete for a package. This can occur if some error was
	// encountered during verification.
	ReasonVerificationIncomplete xpv1.ConditionReason = "SignatureVerificationIncomplete"
	// ReasonVerificationSkipped indicates that signature verification was
	// skipped for a package since no verification configuration was provided.
	ReasonVerificationSkipped xpv1.ConditionReason = "SignatureVerificationSkipped"
	// ReasonVerificationSucceeded indicates that a package's signature has
	// been successfully verified.
	ReasonVerificationSucceeded xpv1.ConditionReason = "SignatureVerificationSucceeded"
	// ReasonVerificationFailed indicates that a package's signature
	// verification failed.
	ReasonVerificationFailed xpv1.ConditionReason = "SignatureVerificationFailed"
)

// Unpacking indicates that the package manager is waiting for a package
// revision to be unpacked.
func Unpacking() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnpacking,
	}
}

// Inactive indicates that the package manager is waiting for a package
// revision to be transitioned to an active state.
func Inactive() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInactive,
	}
}

// Active indicates that the package manager has installed and activated
// a package revision.
func Active() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonActive,
	}
}

// Unhealthy indicates that the current revision is unhealthy.
func Unhealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnhealthy,
	}
}

// Healthy indicates that the current revision is healthy.
func Healthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonHealthy,
	}
}

// UnknownHealth indicates that the health of the current revision is unknown.
func UnknownHealth() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnknownHealth,
	}
}

// AwaitingVerification indicates that the package revision reconciler is
// waiting for a package's signature to be verified.
func AwaitingVerification() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRevisionHealthy,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonAwaitingVerification,
	}
}

// RevisionUnhealthy indicates that the current package revision is unhealthy.
func RevisionUnhealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRevisionHealthy,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnhealthy,
	}
}

// RevisionHealthy indicates that the current package revision is healthy.
func RevisionHealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRevisionHealthy,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonHealthy,
	}
}

// RevisionUnknownHealth indicates that the health of the current package revision is unknown.
func RevisionUnknownHealth() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRevisionHealthy,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnknownHealth,
	}
}

// RuntimeUnhealthy indicates that the current package revision runtime is unhealthy.
func RuntimeUnhealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRuntimeHealthy,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnhealthy,
	}
}

// RuntimeHealthy indicates that the current package revision runtime is healthy.
func RuntimeHealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRuntimeHealthy,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonHealthy,
	}
}

// RuntimeUnknownHealth indicates that the health of the current package revision runtime is unknown.
func RuntimeUnknownHealth() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeRuntimeHealthy,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnknownHealth,
	}
}

// VerificationSucceeded returns a condition indicating that a package's
// signature has been successfully verified using the supplied image config.
func VerificationSucceeded(imageConfig string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeVerified,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonVerificationSucceeded,
		Message:            fmt.Sprintf("Signature verification succeeded using ImageConfig named %q", imageConfig),
	}
}

// VerificationFailed returns a condition indicating that a package's
// signature verification failed using the supplied image config.
func VerificationFailed(imageConfig string, err error) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeVerified,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonVerificationFailed,
		Message:            fmt.Sprintf("Signature verification failed using ImageConfig named %q: %v", imageConfig, err),
	}
}

// VerificationSkipped returns a condition indicating that signature
// verification was skipped for a package.
func VerificationSkipped() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeVerified,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonVerificationSkipped,
	}
}

// VerificationIncomplete returns a condition indicating that signature
// verification is not yet complete for a package.
func VerificationIncomplete(err error) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeVerified,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonVerificationIncomplete,
		Message:            fmt.Sprintf("Error occurred during signature verification %s", err),
	}
}

// PackageHealth returns the health condition of a Package based on the provided
// PackageRevision. It checks both the revision health and runtime health
// conditions, and returns a healthy condition if both are healthy, an unhealthy
// condition if either is unhealthy or an unknown health condition if the
// health status is not clear.
func PackageHealth(pr PackageRevision) xpv1.Condition {
	revisionHealth := pr.GetCondition(TypeRevisionHealthy)
	runtimeHealth := pr.GetCondition(TypeRuntimeHealthy)

	revisionHealthy := revisionHealth.Status == corev1.ConditionTrue
	runtimeHealthy := runtimeHealth.Status == corev1.ConditionTrue
	if _, hasRuntime := pr.(PackageRevisionWithRuntime); !hasRuntime {
		// If the package revision does not have a runtime, we skip checking the runtime health.
		runtimeHealthy = true
	}

	if revisionHealthy && runtimeHealthy {
		return Healthy()
	}

	if !revisionHealthy {
		m := fmt.Sprintf("Package revision health is %q", revisionHealth.Status)
		if revisionHealth.Message != "" {
			m += " with message: " + revisionHealth.Message
		}
		return Unhealthy().WithMessage(m)
	}

	if !runtimeHealthy {
		m := fmt.Sprintf("Package runtime health is %q", runtimeHealth.Status)
		if runtimeHealth.Message != "" {
			m += " with message: " + runtimeHealth.Message
		}
		return Unhealthy().WithMessage(m)
	}

	return UnknownHealth()
}

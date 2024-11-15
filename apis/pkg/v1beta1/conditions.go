package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

const (
	// TypeResolved is the type for the Resolved condition.
	TypeResolved xpv1.ConditionType = "Resolved"
)

// Reasons dependency resolution can fail.
const (
	ReasonFailed    xpv1.ConditionReason = "DependencyResolutionFailed"
	ReasonSucceeded xpv1.ConditionReason = "DependencyResolutionSucceeded"
)

// ResolutionFailed indicates that the dependency resolution process failed.
func ResolutionFailed(err error) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonFailed,
		Message:            fmt.Sprintf("Error occurred during dependency resolution %s", err),
	}
}

// ResolutionSucceeded indicates that the dependency resolution process succeeded.
func ResolutionSucceeded() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonSucceeded,
	}
}

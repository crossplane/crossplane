package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

const (
	// TypeResolved is the type for the Resolved condition.
	TypeResolved xpv2.ConditionType = "Resolved"
)

// Reasons dependency resolution can fail.
const (
	ReasonFailed    xpv2.ConditionReason = "DependencyResolutionFailed"
	ReasonSucceeded xpv2.ConditionReason = "DependencyResolutionSucceeded"
)

// ResolutionFailed indicates that the dependency resolution process failed.
func ResolutionFailed(err error) xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonFailed,
		Message:            fmt.Sprintf("Error occurred during dependency resolution %s", err),
	}
}

// ResolutionSucceeded indicates that the dependency resolution process succeeded.
func ResolutionSucceeded() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonSucceeded,
	}
}

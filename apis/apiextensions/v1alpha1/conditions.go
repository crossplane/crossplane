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

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// Condition types.
const (
	// TypeReady resources are believed to be ready to handle work.
	TypeEstablished runtimev1alpha1.ConditionType = "Established"
)

// Reasons a resource is or is not ready.
const (
	ReasonStarting runtimev1alpha1.ConditionReason = "Creating CRD and starting controller"
	ReasonStarted  runtimev1alpha1.ConditionReason = "Created CRD and started controller"
	ReasonDeleting runtimev1alpha1.ConditionReason = "Definition is being deleted"
)

// Starting returns a condition that indicates a definition or publication is
// establishing its CustomResourceDefinition and starting its controller.
func Starting() runtimev1alpha1.Condition {
	return runtimev1alpha1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonStarting,
	}
}

// Started returns a condition that indicates a definition or publication has
// established its CustomResourceDefinition and started its controller.
func Started() runtimev1alpha1.Condition {
	return runtimev1alpha1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonStarted,
	}
}

// Deleting returns a condition that indicates a definition or publication is
// being deleted.
func Deleting() runtimev1alpha1.Condition {
	return runtimev1alpha1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonDeleting,
	}
}

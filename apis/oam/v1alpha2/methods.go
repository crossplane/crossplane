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

// This code is manually implemented, but should be generated in the future.

package v1alpha2

import (
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

var _ resource.Trait = &ManualScalerTrait{}
var _ resource.Workload = &ContainerizedWorkload{}

// GetCondition of this ManualScalerTrait.
func (tr *ManualScalerTrait) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return tr.Status.GetCondition(ct)
}

// SetConditions of this ManualScalerTrait.
func (tr *ManualScalerTrait) SetConditions(c ...runtimev1alpha1.Condition) {
	tr.Status.SetConditions(c...)
}

// GetWorkloadReference of this ManualScalerTrait.
func (tr *ManualScalerTrait) GetWorkloadReference() runtimev1alpha1.TypedReference {
	return tr.Spec.WorkloadReference
}

// SetWorkloadReference of this ManualScalerTrait.
func (tr *ManualScalerTrait) SetWorkloadReference(r runtimev1alpha1.TypedReference) {
	tr.Spec.WorkloadReference = r
}

// GetCondition of this ApplicationConfiguration.
func (ac *ApplicationConfiguration) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return ac.Status.GetCondition(ct)
}

// SetConditions of this ApplicationConfiguration.
func (ac *ApplicationConfiguration) SetConditions(c ...runtimev1alpha1.Condition) {
	ac.Status.SetConditions(c...)
}

// GetCondition of this Component.
func (cm *Component) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return cm.Status.GetCondition(ct)
}

// SetConditions of this Component.
func (cm *Component) SetConditions(c ...runtimev1alpha1.Condition) {
	cm.Status.SetConditions(c...)
}

// GetCondition of this ContainerizedWorkload.
func (wl *ContainerizedWorkload) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return wl.Status.GetCondition(ct)
}

// SetConditions of this ContainerizedWorkload.
func (wl *ContainerizedWorkload) SetConditions(c ...runtimev1alpha1.Condition) {
	wl.Status.SetConditions(c...)
}

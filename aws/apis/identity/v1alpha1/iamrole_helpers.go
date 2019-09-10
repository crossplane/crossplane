/*
Copyright 2019 The Crossplane Authors.

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
	"github.com/aws/aws-sdk-go-v2/service/iam"
	corev1 "k8s.io/api/core/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

// SetBindingPhase of this IAMRole.
func (r *IAMRole) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	r.Status.SetBindingPhase(p)
}

// GetBindingPhase of this IAMRole.
func (r *IAMRole) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return r.Status.GetBindingPhase()
}

// SetConditions of this IAMRole.
func (r *IAMRole) SetConditions(c ...runtimev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// SetClaimReference of this IAMRole.
func (r *IAMRole) SetClaimReference(ref *corev1.ObjectReference) {
	r.Spec.ClaimReference = ref
}

// GetClaimReference of this IAMRole.
func (r *IAMRole) GetClaimReference() *corev1.ObjectReference {
	return r.Spec.ClaimReference
}

// SetClassReference of this IAMRole.
func (r *IAMRole) SetClassReference(ref *corev1.ObjectReference) {
	r.Spec.ClassReference = ref
}

// GetClassReference of this IAMRole.
func (r *IAMRole) GetClassReference() *corev1.ObjectReference {
	return r.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this IAMRole.
func (r *IAMRole) SetWriteConnectionSecretToReference(ref corev1.LocalObjectReference) {
	r.Spec.WriteConnectionSecretToReference = ref
}

// GetWriteConnectionSecretToReference of this IAMRole.
func (r *IAMRole) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return r.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this IAMRole.
func (r *IAMRole) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return r.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this IAMRole.
func (r *IAMRole) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	r.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object, given the observation
func (r *IAMRole) UpdateExternalStatus(observation iam.Role) {
	r.Status.IAMRoleExternalStatus = IAMRoleExternalStatus{
		ARN:    aws.StringValue(observation.Arn),
		RoleID: aws.StringValue(observation.RoleId),
	}
}

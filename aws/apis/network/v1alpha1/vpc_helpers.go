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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	corev1 "k8s.io/api/core/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// SetBindingPhase of this VPC.
func (v *VPC) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	v.Status.SetBindingPhase(p)
}

// GetBindingPhase of this VPC.
func (v *VPC) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return v.Status.GetBindingPhase()
}

// SetConditions of this VPC.
func (v *VPC) SetConditions(c ...runtimev1alpha1.Condition) {
	v.Status.SetConditions(c...)
}

// SetClaimReference of this VPC.
func (v *VPC) SetClaimReference(r *corev1.ObjectReference) {
	v.Spec.ClaimReference = r
}

// GetClaimReference of this VPC.
func (v *VPC) GetClaimReference() *corev1.ObjectReference {
	return v.Spec.ClaimReference
}

// SetClassReference of this VPC.
func (v *VPC) SetClassReference(r *corev1.ObjectReference) {
	v.Spec.ClassReference = r
}

// GetClassReference of this VPC.
func (v *VPC) GetClassReference() *corev1.ObjectReference {
	return v.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this VPC.
func (v *VPC) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	v.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this VPC.
func (v *VPC) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return v.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this VPC.
func (v *VPC) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return v.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this VPC.
func (v *VPC) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	v.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object,  given the observation
func (v *VPC) UpdateExternalStatus(observation ec2.Vpc) {
	v.Status.VPCExternalStatus = VPCExternalStatus{
		VPCID:    aws.StringValue(observation.VpcId),
		Tags:     BuildFromEC2Tags(observation.Tags),
		VPCState: string(observation.State),
	}
}

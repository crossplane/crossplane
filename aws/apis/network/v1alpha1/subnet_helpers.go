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

// SetBindingPhase of this Subnet.
func (s *Subnet) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	s.Status.SetBindingPhase(p)
}

// GetBindingPhase of this Subnet.
func (s *Subnet) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return s.Status.GetBindingPhase()
}

// SetConditions of this Subnet.
func (s *Subnet) SetConditions(c ...runtimev1alpha1.Condition) {
	s.Status.SetConditions(c...)
}

// SetClaimReference of this Subnet.
func (s *Subnet) SetClaimReference(r *corev1.ObjectReference) {
	s.Spec.ClaimReference = r
}

// GetClaimReference of this Subnet.
func (s *Subnet) GetClaimReference() *corev1.ObjectReference {
	return s.Spec.ClaimReference
}

// SetClassReference of this Subnet.
func (s *Subnet) SetClassReference(r *corev1.ObjectReference) {
	s.Spec.ClassReference = r
}

// GetClassReference of this Subnet.
func (s *Subnet) GetClassReference() *corev1.ObjectReference {
	return s.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this Subnet.
func (s *Subnet) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	s.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this Subnet.
func (s *Subnet) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return s.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this Subnet.
func (s *Subnet) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return s.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this Subnet.
func (s *Subnet) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	s.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object,  given the observation
func (s *Subnet) UpdateExternalStatus(observation ec2.Subnet) {
	s.Status.SubnetExternalStatus = SubnetExternalStatus{
		SubnetID:    aws.StringValue(observation.SubnetId),
		Tags:        BuildFromEC2Tags(observation.Tags),
		SubnetState: string(observation.State),
	}
}

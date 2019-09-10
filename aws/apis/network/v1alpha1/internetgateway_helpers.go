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

// SetBindingPhase of this InternetGateway.
func (i *InternetGateway) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	i.Status.SetBindingPhase(p)
}

// GetBindingPhase of this InternetGateway.
func (i *InternetGateway) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return i.Status.GetBindingPhase()
}

// SetConditions of this InternetGateway.
func (i *InternetGateway) SetConditions(c ...runtimev1alpha1.Condition) {
	i.Status.SetConditions(c...)
}

// SetClaimReference of this InternetGateway.
func (i *InternetGateway) SetClaimReference(r *corev1.ObjectReference) {
	i.Spec.ClaimReference = r
}

// GetClaimReference of this InternetGateway.
func (i *InternetGateway) GetClaimReference() *corev1.ObjectReference {
	return i.Spec.ClaimReference
}

// SetClassReference of this InternetGateway.
func (i *InternetGateway) SetClassReference(r *corev1.ObjectReference) {
	i.Spec.ClassReference = r
}

// GetClassReference of this InternetGateway.
func (i *InternetGateway) GetClassReference() *corev1.ObjectReference {
	return i.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this InternetGateway.
func (i *InternetGateway) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	i.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this InternetGateway.
func (i *InternetGateway) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return i.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this InternetGateway.
func (i *InternetGateway) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return i.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this InternetGateway.
func (i *InternetGateway) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	i.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object, given the observation
func (i *InternetGateway) UpdateExternalStatus(observation ec2.InternetGateway) {
	attachments := make([]InternetGatewayAttachment, len(observation.Attachments))
	for k, a := range observation.Attachments {
		attachments[k] = InternetGatewayAttachment{
			AttachmentStatus: string(a.State),
			VPCID:            aws.StringValue(a.VpcId),
		}
	}

	i.Status.InternetGatewayExternalStatus = InternetGatewayExternalStatus{
		InternetGatewayID: aws.StringValue(observation.InternetGatewayId),
		Attachments:       attachments,
	}
}

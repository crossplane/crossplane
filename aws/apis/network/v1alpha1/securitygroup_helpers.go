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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	corev1 "k8s.io/api/core/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

// SetBindingPhase of this SecurityGroup.
func (s *SecurityGroup) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	s.Status.SetBindingPhase(p)
}

// GetBindingPhase of this SecurityGroup.
func (s *SecurityGroup) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return s.Status.GetBindingPhase()
}

// SetConditions of this SecurityGroup.
func (s *SecurityGroup) SetConditions(c ...runtimev1alpha1.Condition) {
	s.Status.SetConditions(c...)
}

// SetClaimReference of this SecurityGroup.
func (s *SecurityGroup) SetClaimReference(r *corev1.ObjectReference) {
	s.Spec.ClaimReference = r
}

// GetClaimReference of this SecurityGroup.
func (s *SecurityGroup) GetClaimReference() *corev1.ObjectReference {
	return s.Spec.ClaimReference
}

// SetClassReference of this SecurityGroup.
func (s *SecurityGroup) SetClassReference(r *corev1.ObjectReference) {
	s.Spec.ClassReference = r
}

// GetClassReference of this SecurityGroup.
func (s *SecurityGroup) GetClassReference() *corev1.ObjectReference {
	return s.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this SecurityGroup.
func (s *SecurityGroup) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	s.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this SecurityGroup.
func (s *SecurityGroup) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return s.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this SecurityGroup.
func (s *SecurityGroup) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return s.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this SecurityGroup.
func (s *SecurityGroup) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	s.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object, given the observation
func (s *SecurityGroup) UpdateExternalStatus(observation ec2.SecurityGroup) {
	s.Status.SecurityGroupExternalStatus = SecurityGroupExternalStatus{
		SecurityGroupID: aws.StringValue(observation.GroupId),
		Tags:            BuildFromEC2Tags(observation.Tags),
	}
}

// BuildEC2Permissions converts object Permissions to ec2 format
func BuildEC2Permissions(objectPerms []IPPermission) []ec2.IpPermission {
	permissions := make([]ec2.IpPermission, len(objectPerms))
	for i, p := range objectPerms {

		ipPerm := ec2.IpPermission{
			FromPort:   aws.Int64(int(p.FromPort)),
			ToPort:     aws.Int64(int(p.ToPort)),
			IpProtocol: aws.String(p.IPProtocol),
		}

		ipPerm.IpRanges = make([]ec2.IpRange, len(p.CIDRBlocks))
		for j, c := range p.CIDRBlocks {
			ipPerm.IpRanges[j] = ec2.IpRange{
				CidrIp:      aws.String(c.CIDRIP),
				Description: aws.String(c.Description),
			}
		}

		permissions[i] = ipPerm
	}

	return permissions
}

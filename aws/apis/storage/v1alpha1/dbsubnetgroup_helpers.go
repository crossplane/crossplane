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
	"github.com/aws/aws-sdk-go-v2/service/rds"
	corev1 "k8s.io/api/core/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

// SetBindingPhase of this DBSubnetGroup.
func (b *DBSubnetGroup) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	b.Status.SetBindingPhase(p)
}

// GetBindingPhase of this DBSubnetGroup.
func (b *DBSubnetGroup) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return b.Status.GetBindingPhase()
}

// SetConditions of this DBSubnetGroup.
func (b *DBSubnetGroup) SetConditions(c ...runtimev1alpha1.Condition) {
	b.Status.SetConditions(c...)
}

// SetClaimReference of this DBSubnetGroup.
func (b *DBSubnetGroup) SetClaimReference(r *corev1.ObjectReference) {
	b.Spec.ClaimReference = r
}

// GetClaimReference of this DBSubnetGroup.
func (b *DBSubnetGroup) GetClaimReference() *corev1.ObjectReference {
	return b.Spec.ClaimReference
}

// SetClassReference of this DBSubnetGroup.
func (b *DBSubnetGroup) SetClassReference(r *corev1.ObjectReference) {
	b.Spec.ClassReference = r
}

// GetClassReference of this DBSubnetGroup.
func (b *DBSubnetGroup) GetClassReference() *corev1.ObjectReference {
	return b.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this DBSubnetGroup.
func (b *DBSubnetGroup) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	b.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this DBSubnetGroup.
func (b *DBSubnetGroup) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return b.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this DBSubnetGroup.
func (b *DBSubnetGroup) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return b.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this DBSubnetGroup.
func (b *DBSubnetGroup) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	b.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object, given the observation
func (b *DBSubnetGroup) UpdateExternalStatus(observation rds.DBSubnetGroup) {

	subnets := make([]Subnet, len(observation.Subnets))
	for i, sn := range observation.Subnets {
		subnets[i] = Subnet{
			SubnetID:     aws.StringValue(sn.SubnetIdentifier),
			SubnetStatus: aws.StringValue(sn.SubnetStatus),
		}
	}

	b.Status.DBSubnetGroupExternalStatus = DBSubnetGroupExternalStatus{
		DBSubnetGroupARN:  aws.StringValue(observation.DBSubnetGroupArn),
		SubnetGroupStatus: aws.StringValue(observation.SubnetGroupStatus),
		Subnets:           subnets,
		VPCID:             aws.StringValue(observation.VpcId),
	}
}

// BuildFromRDSTags returns a list of tags, off of the given RDS tags
func BuildFromRDSTags(tags []rds.Tag) []Tag {
	res := make([]Tag, len(tags))
	for i, t := range tags {
		res[i] = Tag{aws.StringValue(t.Key), aws.StringValue(t.Value)}
	}

	return res
}

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

// SetBindingPhase of this RouteTable.
func (t *RouteTable) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	t.Status.SetBindingPhase(p)
}

// GetBindingPhase of this RouteTable.
func (t *RouteTable) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return t.Status.GetBindingPhase()
}

// SetConditions of this RouteTable.
func (t *RouteTable) SetConditions(c ...runtimev1alpha1.Condition) {
	t.Status.SetConditions(c...)
}

// SetClaimReference of this RouteTable.
func (t *RouteTable) SetClaimReference(r *corev1.ObjectReference) {
	t.Spec.ClaimReference = r
}

// GetClaimReference of this RouteTable.
func (t *RouteTable) GetClaimReference() *corev1.ObjectReference {
	return t.Spec.ClaimReference
}

// SetClassReference of this RouteTable.
func (t *RouteTable) SetClassReference(r *corev1.ObjectReference) {
	t.Spec.ClassReference = r
}

// GetClassReference of this RouteTable.
func (t *RouteTable) GetClassReference() *corev1.ObjectReference {
	return t.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this RouteTable.
func (t *RouteTable) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	t.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this RouteTable.
func (t *RouteTable) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return t.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this RouteTable.
func (t *RouteTable) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return t.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this RouteTable.
func (t *RouteTable) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	t.Spec.ReclaimPolicy = p
}

// UpdateExternalStatus updates the external status object, given the observation
func (t *RouteTable) UpdateExternalStatus(observation ec2.RouteTable) {
	st := RouteTableExternalStatus{
		RouteTableID: aws.StringValue(observation.RouteTableId),
		Routes:       []RouteState{},
		Associations: []AssociationState{},
	}

	st.Routes = make([]RouteState, len(observation.Routes))
	for i, rt := range observation.Routes {
		st.Routes[i] = RouteState{
			RouteState: string(rt.State),
			Route: Route{
				DestinationCIDRBlock: aws.StringValue(rt.DestinationCidrBlock),
				GatewayID:            aws.StringValue(rt.GatewayId),
			},
		}
	}

	st.Associations = make([]AssociationState, len(observation.Associations))
	for i, asc := range observation.Associations {
		st.Associations[i] = AssociationState{
			Main:          aws.BoolValue(asc.Main),
			AssociationID: aws.StringValue(asc.RouteTableAssociationId),
			Association: Association{
				SubnetID: aws.StringValue(asc.SubnetId),
			},
		}
	}

	t.Status.RouteTableExternalStatus = st
}

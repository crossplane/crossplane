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

package resource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

type MockBindable struct{ Phase v1alpha1.BindingPhase }

func (m *MockBindable) SetBindingPhase(p v1alpha1.BindingPhase) { m.Phase = p }
func (m *MockBindable) GetBindingPhase() v1alpha1.BindingPhase  { return m.Phase }

type MockConditionSetter struct{ Conditions []v1alpha1.Condition }

func (m *MockConditionSetter) SetConditions(c ...v1alpha1.Condition) { m.Conditions = c }

type MockClaimReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClaimReferencer) SetClaimReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockClaimReferencer) GetClaimReference() *corev1.ObjectReference  { return m.Ref }

type MockClassReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClassReferencer) SetClassReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockClassReferencer) GetClassReference() *corev1.ObjectReference  { return m.Ref }

type MockManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference  { return m.Ref }

type MockConnectionSecretWriterTo struct{ Ref corev1.LocalObjectReference }

func (m *MockConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	m.Ref = r
}
func (m *MockConnectionSecretWriterTo) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return m.Ref
}

type MockReclaimer struct{ Policy v1alpha1.ReclaimPolicy }

func (m *MockReclaimer) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) { m.Policy = p }
func (m *MockReclaimer) GetReclaimPolicy() v1alpha1.ReclaimPolicy  { return m.Policy }

type MockObjectKind struct{ GVK schema.GroupVersionKind }

func (m *MockObjectKind) SetGroupVersionKind(kind schema.GroupVersionKind) { m.GVK = kind }
func (m *MockObjectKind) GroupVersionKind() schema.GroupVersionKind        { return m.GVK }

var _ Claim = &MockClaim{}

type MockClaim struct {
	runtime.Object

	metav1.ObjectMeta
	MockClassReferencer
	MockManagedResourceReferencer
	MockConnectionSecretWriterTo
	MockConditionSetter
	MockBindable
}

func (m *MockClaim) GetObjectKind() schema.ObjectKind {
	return &MockObjectKind{GVK: schema.GroupVersionKind{Group: "mock.crossplane.io", Version: "v1alpha", Kind: "claim"}}
}

var _ Managed = &MockManaged{}

type MockManaged struct {
	runtime.Object

	metav1.ObjectMeta
	MockClassReferencer
	MockClaimReferencer
	MockConnectionSecretWriterTo
	MockReclaimer
	MockConditionSetter
	MockBindable
}

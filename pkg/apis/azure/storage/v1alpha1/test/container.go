/*
Copyright 2018 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance With the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// MockContainer builder to create a continer object for testing
type MockContainer struct {
	*v1alpha1.Container
}

// NewMockContainer new container builcer
func NewMockContainer(ns, name string) *MockContainer {
	return &MockContainer{
		Container: &v1alpha1.Container{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// WithTypeMeta sets TypeMeta value
func (tc *MockContainer) WithTypeMeta(tm metav1.TypeMeta) *MockContainer {
	tc.TypeMeta = tm
	return tc
}

// WithObjectMeta sets ObjectMeta value
func (tc *MockContainer) WithObjectMeta(om metav1.ObjectMeta) *MockContainer {
	tc.ObjectMeta = om
	return tc
}

// WithUID sets UID value
func (tc *MockContainer) WithUID(uid string) *MockContainer {
	tc.ObjectMeta.UID = types.UID(uid)
	return tc
}

// WithDeleteTimestamp sets deletion timestamp value
func (tc *MockContainer) WithDeleteTimestamp(t time.Time) *MockContainer {
	tc.Container.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	return tc
}

// WithFinalizer sets finalizer
func (tc *MockContainer) WithFinalizer(f string) *MockContainer {
	tc.Container.ObjectMeta.Finalizers = append(tc.Container.ObjectMeta.Finalizers, f)
	return tc
}

// WithFinalizers sets finalizers list
func (tc *MockContainer) WithFinalizers(f []string) *MockContainer {
	tc.Container.ObjectMeta.Finalizers = f
	return tc
}

// WithSpecClassRef set class reference
func (tc *MockContainer) WithSpecClassRef(ref *corev1.ObjectReference) *MockContainer {
	tc.Spec.ClassRef = ref
	return tc
}

// WithSpecClaimRef set class reference
func (tc *MockContainer) WithSpecClaimRef(ref *corev1.ObjectReference) *MockContainer {
	tc.Spec.ClaimRef = ref
	return tc
}

// WithSpecAccountRef sets spec account reference value
func (tc *MockContainer) WithSpecAccountRef(name string) *MockContainer {
	tc.Container.Spec.AccountRef = corev1.LocalObjectReference{Name: name}
	return tc
}

// WithSpecNameFormat sets spec name format
func (tc *MockContainer) WithSpecNameFormat(f string) *MockContainer {
	tc.Container.Spec.NameFormat = f
	return tc
}

// WithSpecReclaimPolicy sets spec reclaim policy value
func (tc *MockContainer) WithSpecReclaimPolicy(p corev1alpha1.ReclaimPolicy) *MockContainer {
	tc.Container.Spec.ReclaimPolicy = p
	return tc
}

// WithSpecPAC sets spec public access type value
func (tc *MockContainer) WithSpecPAC(pac azblob.PublicAccessType) *MockContainer {
	tc.Container.Spec.PublicAccessType = pac
	return tc
}

// WithSpecMetadata sets spec metadata value
func (tc *MockContainer) WithSpecMetadata(meta map[string]string) *MockContainer {
	tc.Container.Spec.Metadata = meta
	return tc
}

// WithStatusSetBound set status bound state
func (tc *MockContainer) WithStatusSetBound(bound bool) *MockContainer {
	tc.Status.SetBound(bound)
	return tc
}

// WithFailedCondition sets status failed condition
func (tc *MockContainer) WithFailedCondition(reason, msg string) *MockContainer {
	tc.Status.SetFailed(reason, msg)
	return tc
}

// WithUnsetAllConditions resets all status conditions
func (tc *MockContainer) WithUnsetAllConditions() *MockContainer {
	tc.Status.UnsetAllConditions()
	return tc
}

// WithReadyCondition sets status ready condition
func (tc *MockContainer) WithReadyCondition() *MockContainer {
	tc.Status.SetReady()
	return tc
}

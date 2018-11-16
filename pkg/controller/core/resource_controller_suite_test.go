/*
Copyright 2018 The Crossplane Authors.

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

package core

import (
	"context"
	"flag"
	"testing"

	"github.com/crossplaneio/crossplane/pkg/apis/core"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	name      = "test-resource"
)

var (
	cfg *rest.Config
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	core.AddToScheme(scheme.Scheme)

	t := test.NewTestEnv(namespace, test.CRDs())
	cfg = t.Start()
	t.StopAndExit(m.Run())
}

//---------------------------------------------------------------------------------------------------------------------
// testAbstractInstance

type testAbstractInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   testAbstractInstanceSpec            `json:"spec,omitempty"`
	Status corev1alpha1.AbstractResourceStatus `json:"status,omitempty"`
}

type testAbstractInstanceSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`
}

func testInstance() *testAbstractInstance {
	return &testAbstractInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ObjectReference to using this object as a reference
func (t *testAbstractInstance) ObjectReference() *corev1.ObjectReference {
	if t.Kind == "" {
		t.Kind = "testAbstractInstance"
	}
	if t.APIVersion == "" {
		t.APIVersion = "core.crossplane.io/v1alpha1"
	}
	return &corev1.ObjectReference{
		APIVersion: t.APIVersion,
		Kind:       t.Kind,
		Name:       t.Name,
		Namespace:  t.Namespace,
		UID:        t.UID,
	}
}

// DeepCopyObject is a fake/stub implementation simply to satisfy the runtime.Object interface for
// this test only type
func (t *testAbstractInstance) DeepCopyObject() runtime.Object {
	return t
}

// OwnerReference to use this object as an owner
func (t *testAbstractInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(t.ObjectReference())
}

func (t *testAbstractInstance) ResourceStatus() *corev1alpha1.AbstractResourceStatus {
	return &t.Status
}

func (t *testAbstractInstance) GetObjectMeta() *metav1.ObjectMeta {
	return &t.ObjectMeta
}

func (t *testAbstractInstance) ClassRef() *corev1.ObjectReference {
	return t.Spec.ClassRef
}

func (t *testAbstractInstance) ResourceRef() *corev1.ObjectReference {
	return t.Spec.ResourceRef
}

func (t *testAbstractInstance) SetResourceRef(ref *corev1.ObjectReference) {
	t.Spec.ResourceRef = ref
}

//---------------------------------------------------------------------------------------------------------------------
// Mock objects

// MockClient controller-runtime client
type MockClient struct {
	client.Client

	MockGet    func(...interface{}) error
	MockUpdate func(...interface{}) error
}

func (mc *MockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return mc.MockGet(ctx, key, obj)
}

func (mc *MockClient) Update(ctx context.Context, obj runtime.Object) error {
	return mc.MockUpdate(ctx, obj)
}

// MockRecorder Kubernetes events recorder
type MockRecorder struct {
	record.EventRecorder
}

// The resulting event will be created in the same namespace as the reference object.
func (mr *MockRecorder) Event(object runtime.Object, eventtype, reason, message string) {}

// Eventf is just like Event, but with Sprintf for the message field.
func (mr *MockRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}

// PastEventf is just like Eventf, but with an option to specify the event's 'timestamp' field.
func (mr *MockRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
}

// AnnotatedEventf is just like eventf, but with annotations attached
func (mr *MockRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

type MockResourceHandler struct {
	MockProvision     func(*corev1alpha1.ResourceClass, corev1alpha1.AbstractResource, client.Client) (corev1alpha1.ConcreteResource, error)
	MockFind          func(types.NamespacedName, client.Client) (corev1alpha1.ConcreteResource, error)
	MockSetBindStatus func(types.NamespacedName, client.Client, bool) error
}

func (mrh *MockResourceHandler) Provision(class *corev1alpha1.ResourceClass, instance corev1alpha1.AbstractResource, c client.Client) (corev1alpha1.ConcreteResource, error) {
	return mrh.MockProvision(class, instance, c)
}

func (mrh *MockResourceHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.ConcreteResource, error) {
	return mrh.MockFind(n, c)
}

func (mrh *MockResourceHandler) SetBindStatus(n types.NamespacedName, c client.Client, s bool) error {
	return mrh.MockSetBindStatus(n, c, s)
}

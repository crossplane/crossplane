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
	"fmt"
	"testing"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	. "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	handlers = map[string]ResourceHandler{}
)

func TestProvision(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	h := &MockResourceHandler{}
	i := testInstance()

	// test: without ResourceClass definition - expected to: fail
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))

	// test: ResourceClass is not found - expected to: fail
	i.Spec.ClassRef = &corev1.ObjectReference{
		Name:      "foo",
		Namespace: "system",
	}
	mc.MockGet = func(...interface{}) error {
		return fmt.Errorf("not-found")
	}
	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))

	// test: ResourceClass has no provisioner information
	mc.MockGet = func(args ...interface{}) error { return nil }
	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))

	// test: ResourceClass has test provisioner, but provisioning failed
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	handlers["test-provisioner"] = h
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp corev1alpha1.AbstractResource, cl client.Client) (corev1alpha1.ConcreteResource, error) {
		return nil, fmt.Errorf("test-provisioning-error")
	}

	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))

	// test: ResourceClass has RDS provisioner, no provisioning failures
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp corev1alpha1.AbstractResource, cl client.Client) (corev1alpha1.ConcreteResource, error) {
		return &corev1alpha1.BasicResource{}, nil
	}
	r.bind = func(corev1alpha1.AbstractResource) (reconcile.Result, error) {
		return Result, nil
	}

	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
}

func TestBind(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	i := testInstance()
	h := &MockResourceHandler{}

	// test bind - handler is not found
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	c := i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))

	// test bind - error  retrieving resource instance
	i.Status.Provisioner = "test"
	handlers["test"] = h
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.ConcreteResource, error) {
		return nil, fmt.Errorf("test-error")
	}
	i.Spec.ResourceRef = &corev1.ObjectReference{
		Namespace: "foo",
		Name:      "bar",
	}
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceInstance))

	// resource is not available
	br := corev1alpha1.NewBasicResource(nil, "", "", "not-available")
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.ConcreteResource, error) {
		return br, nil
	}
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceInstance))
	c = i.Status.Condition(corev1alpha1.Pending)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(waitResourceIsNotAvailable))

	// error retrieving resource secret
	br = corev1alpha1.NewBasicResource(
		&corev1.ObjectReference{
			Name:      "test-resource",
			Namespace: "default",
		}, "test-secret", "test-endpoint", "available")

	mk := fake.NewSimpleClientset(&corev1.Secret{})
	r.kubeclient = mk

	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceSecret))

	// error applying instance secret
	sec := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}
	mk = fake.NewSimpleClientset(sec)
	mk.PrependReactor("create", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test-error-create")
	})
	r.kubeclient = mk
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorApplyingInstanceSecret))

	// bind
	mk = fake.NewSimpleClientset(sec)
	r.kubeclient = mk
	mc.MockUpdate = func(...interface{}) error { return nil }
	h.MockSetBindStatus = func(namespacedName types.NamespacedName, i client.Client, b bool) error { return nil }
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
	c = i.Status.Condition(corev1alpha1.Ready)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(i.Status.BindingStatusPhase.Phase).To(Equal(corev1alpha1.BindingStateBound))
}

func TestDelete(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	i := testInstance()
	h := &MockResourceHandler{}

	// test bind - handler is not found
	handlers["test"] = h
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._delete(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	c := i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))

	// test delete successful
	i.Status.Provisioner = "test"
	i.Spec.ResourceRef = &corev1.ObjectReference{
		Namespace: "default",
		Name:      "test-resource",
	}
	bindingStatus := true
	h.MockSetBindStatus = func(namespacedName types.NamespacedName, i client.Client, b bool) error {
		bindingStatus = b
		return nil
	}
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err = r._delete(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
	c = i.Status.Condition(corev1alpha1.Deleting)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(len(i.Finalizers)).To(Equal(0))
	g.Expect(bindingStatus).To(BeFalse())
}

func TestReconciler_Reconcile(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	i := testInstance()

	// 1) reconcile deleted instance
	// mocked happy delete function
	deleteCalled := false
	deleteFunc := func(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
		deleteCalled = true
		return Result, nil
	}
	tm := v1.Now()
	i.DeletionTimestamp = &tm
	r.delete = deleteFunc
	rs, err := r._reconcile(i)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(deleteCalled).To(BeTrue())

	// 2) add finalizer failed
	mc.MockGet = func(...interface{}) error {
		return nil
	}
	mc.MockUpdate = func(...interface{}) error {
		return fmt.Errorf("test-error")
	}
	r.delete = nil            // clear out the mocked delete func
	i.DeletionTimestamp = nil // clear out the deletion timestamp
	rs, err = r._reconcile(i)
	g.Expect(rs).To(Equal(ResultRequeue))
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(Equal("test-error"))

	// 3) provision path
	// mocked happy provision function
	provisionCalled := false
	provisionFunc := func(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
		provisionCalled = true
		return Result, nil
	}
	mc.MockGet = func(...interface{}) error {
		return nil
	}
	mc.MockUpdate = func(...interface{}) error {
		return nil
	}
	r.provision = provisionFunc
	rs, err = r._reconcile(i)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(provisionCalled).To(BeTrue())

	// 4) bind path
	// mocked happy bind function
	bindCalled := false
	bindFunc := func(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
		bindCalled = true
		return Result, nil
	}
	// give the instance a finalizer and a resource ref so that we'll take the bind codepath
	i.Finalizers = append(i.Finalizers, "finalizer.resourcecontroller.core.crossplane.io")
	i.Spec.ResourceRef = &corev1.ObjectReference{}
	mc.MockUpdate = nil
	r.provision = nil
	bindCalled = false
	r.bind = bindFunc
	rs, err = r._reconcile(i)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(bindCalled).To(BeTrue())
}

func TestResolveClassInstanceValues(t *testing.T) {
	g := NewGomegaWithT(t)

	f := func(cv, iv, expV string, expErr bool) {
		v, err := ResolveClassInstanceValues(cv, iv)
		g.Expect(v).To(Equal(expV))
		if expErr {
			g.Expect(err).To(HaveOccurred())
			g.Expect(err).To(MatchError(fmt.Errorf("mysql instance value [%s] does not match the one defined in the resource class [%s]", iv, cv)))
		} else {
			g.Expect(err).NotTo(HaveOccurred())
		}
	}

	f("", "", "", false)
	f("a", "", "a", false)
	f("", "b", "b", false)
	f("ab", "ab", "ab", false)
	f("ab", "ba", "", true)
}

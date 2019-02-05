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

func TestGetHandler(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	claim := testClaim()

	// test: claim has no ResourceClass
	h, err := r._getHandler(claim)
	g.Expect(err).To(HaveOccurred())
	g.Expect(h).To(BeNil())

	// resource class exists and has provisioner info, but its an unknown provisioner
	// that's not an error, we'll just ignore and let an external provisioner handle it
	claim.Spec.ClassRef = &corev1.ObjectReference{
		Name:      "foo",
		Namespace: "system",
	}
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	h, err = r._getHandler(claim)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(h).To(BeNil())

	// resource class has a known provisioner, it should be returned
	handlers["test-provisioner"] = &MockResourceHandler{}
	h, err = r._getHandler(claim)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(h).NotTo(BeNil())

	// the claim already has a provisioner saved on its resource status,
	// the handler should be returned
	claim.ClaimStatus().Provisioner = "test-provisioner"
	mc.MockGet = nil // Get should not be called on this path
	h, err = r._getHandler(claim)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(h).NotTo(BeNil())
}

func TestProvision(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	h := &MockResourceHandler{}
	claim := testClaim()

	// test: without ResourceClass definition - expected to: fail
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._provision(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorRetrievingResourceClass)

	// test: ResourceClass is not found - expected to: fail
	claim.Spec.ClassRef = &corev1.ObjectReference{
		Name:      "foo",
		Namespace: "system",
	}
	mc.MockGet = func(...interface{}) error {
		return fmt.Errorf("not-found")
	}
	rs, err = r._provision(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorRetrievingResourceClass)

	// test: ResourceClass has test provisioner, but provisioning failed
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp corev1alpha1.ResourceClaim, cl client.Client) (corev1alpha1.Resource, error) {
		return nil, fmt.Errorf("test-provisioning-error")
	}
	rs, err = r._provision(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorResourceProvisioning)

	// test: ResourceClass has test provisioner, no provisioning failures
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp corev1alpha1.ResourceClaim, cl client.Client) (corev1alpha1.Resource, error) {
		return &corev1alpha1.BasicResource{}, nil
	}
	r.bind = func(corev1alpha1.ResourceClaim, ResourceHandler) (reconcile.Result, error) {
		return Result, nil
	}

	rs, err = r._provision(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
}

func TestBind(t *testing.T) {
	mc := &MockClient{}
	mc.MockUpdate = func(...interface{}) error { return nil }

	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	claim := testClaim()
	h := &MockResourceHandler{}

	// test bind - error retrieving resource
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.Resource, error) {
		return nil, fmt.Errorf("test-error")
	}
	claim.Spec.ResourceRef = &corev1.ObjectReference{
		Namespace: "foo",
		Name:      "bar",
	}
	rs, err := r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorRetrievingResource)

	// resource is not available
	br := corev1alpha1.NewBasicResource(nil, "", "", "not-available")
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.Resource, error) {
		return br, nil
	}
	rs, err = r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionUnset(g, claim, corev1alpha1.Failed, errorRetrievingResource)
	assertConditionSet(g, claim, corev1alpha1.Pending, waitResourceIsNotAvailable)

	// error retrieving resource secret
	br = corev1alpha1.NewBasicResource(
		&corev1.ObjectReference{
			Name:      "test-resource",
			Namespace: "default",
		}, "test-secret", "test-endpoint", "available")

	mk := fake.NewSimpleClientset(&corev1.Secret{})
	r.kubeclient = mk

	rs, err = r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorRetrievingResourceSecret)

	// error applying resource secret
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
	rs, err = r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorApplyingResourceSecret)

	// failure to set binding status
	mk = fake.NewSimpleClientset(sec)
	r.kubeclient = mk
	h.MockSetBindStatus = func(namespacedName types.NamespacedName, i client.Client, b bool) error {
		return fmt.Errorf("test-error-set-bind-status")
	}
	rs, err = r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorSettingResourceBindStatus)

	// bind
	mk = fake.NewSimpleClientset(sec)
	r.kubeclient = mk
	h.MockSetBindStatus = func(namespacedName types.NamespacedName, i client.Client, b bool) error { return nil }
	rs, err = r._bind(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	assertConditionUnset(g, claim, corev1alpha1.Failed, errorSettingResourceBindStatus)
	assertConditionSet(g, claim, corev1alpha1.Ready, "")
	g.Expect(claim.Status.CredentialsSecretRef.Name).To(Equal(claim.Name))
	g.Expect(claim.Status.BindingStatusPhase.Phase).To(Equal(corev1alpha1.BindingStateBound))
}

func TestDelete(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}, handlers: handlers}
	claim := testClaim()
	h := &MockResourceHandler{}

	// test delete successful
	claim.Spec.ResourceRef = &corev1.ObjectReference{
		Namespace: "default",
		Name:      "test-resource",
	}
	bindingStatus := true
	h.MockSetBindStatus = func(namespacedName types.NamespacedName, i client.Client, b bool) error {
		bindingStatus = b
		return nil
	}
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._delete(claim, h)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(Result))
	c := claim.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).To(BeNil())
	assertConditionSet(g, claim, corev1alpha1.Deleting, "")
	g.Expect(len(claim.Finalizers)).To(Equal(0))
	g.Expect(bindingStatus).To(BeFalse())
}

func TestReconciler_Reconcile(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{
		Client:   mc,
		recorder: &MockRecorder{},
		handlers: handlers,
	}
	claim := testClaim()

	// 1) getHandler returns an error, failure condition should be set
	r.getHandler = func(claim corev1alpha1.ResourceClaim) (ResourceHandler, error) {
		return &MockResourceHandler{}, fmt.Errorf("mocked getHandler error")
	}
	mc.MockUpdate = func(...interface{}) error {
		return nil
	}
	rs, err := r._reconcile(claim)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rs).To(Equal(ResultRequeue))
	assertConditionSet(g, claim, corev1alpha1.Failed, errorRetrievingHandler)

	// 2) getHandler does not return an error, but also doesn't find a known handler
	// this is OK since an external provisioner may handle it instead
	// we should not requeue
	r.getHandler = func(claim corev1alpha1.ResourceClaim) (ResourceHandler, error) {
		return nil, nil
	}
	rs, err = r._reconcile(claim)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rs).To(Equal(Result))

	// 3) reconcile deleted resource
	// mocked happy delete function
	deleteCalled := false
	deleteFunc := func(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
		deleteCalled = true
		return Result, nil
	}
	r.getHandler = func(claim corev1alpha1.ResourceClaim) (ResourceHandler, error) {
		return &MockResourceHandler{}, nil
	}
	tm := v1.Now()
	claim.DeletionTimestamp = &tm
	r.delete = deleteFunc
	rs, err = r._reconcile(claim)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(deleteCalled).To(BeTrue())

	// 4) add finalizer failed
	mc.MockGet = func(...interface{}) error {
		return nil
	}
	mc.MockUpdate = func(...interface{}) error {
		return fmt.Errorf("test-error")
	}
	r.delete = nil                // clear out the mocked delete func
	claim.DeletionTimestamp = nil // clear out the deletion timestamp
	rs, err = r._reconcile(claim)
	g.Expect(rs).To(Equal(ResultRequeue))
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(Equal("test-error"))

	// 5) provision path
	// mocked happy provision function
	provisionCalled := false
	provisionFunc := func(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
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
	rs, err = r._reconcile(claim)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(provisionCalled).To(BeTrue())

	// 6) bind path
	// mocked happy bind function
	bindCalled := false
	bindFunc := func(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
		bindCalled = true
		return Result, nil
	}
	// give the resource a finalizer and a resource ref so that we'll take the bind codepath
	claim.Finalizers = append(claim.Finalizers, "finalizer.resourcecontroller.core.crossplane.io")
	claim.Spec.ResourceRef = &corev1.ObjectReference{}
	mc.MockUpdate = nil
	r.provision = nil
	bindCalled = false
	r.bind = bindFunc
	rs, err = r._reconcile(claim)
	g.Expect(rs).To(Equal(Result))
	g.Expect(err).To(BeNil())
	g.Expect(bindCalled).To(BeTrue())
}

func TestResolveClassClaimValues(t *testing.T) {
	g := NewGomegaWithT(t)

	f := func(cv, iv, expV string, expErr bool) {
		v, err := ResolveClassClaimValues(cv, iv)
		g.Expect(v).To(Equal(expV))
		if expErr {
			g.Expect(err).To(HaveOccurred())
			g.Expect(err).To(MatchError(fmt.Errorf("claim value [%s] does not match the one defined in the resource class [%s]", iv, cv)))
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

func assertConditionSet(g *GomegaWithT, claim corev1alpha1.ResourceClaim, cType corev1alpha1.ConditionType, expectedReason string) {
	assertCondition(g, claim, cType, corev1.ConditionTrue, expectedReason)
}

func assertConditionUnset(g *GomegaWithT, claim corev1alpha1.ResourceClaim, cType corev1alpha1.ConditionType, expectedReason string) {
	assertCondition(g, claim, cType, corev1.ConditionFalse, expectedReason)
}

func assertCondition(g *GomegaWithT, claim corev1alpha1.ResourceClaim, cType corev1alpha1.ConditionType,
	expectedStatus corev1.ConditionStatus, expectedReason string) {

	c := claim.ClaimStatus().Condition(cType)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(expectedStatus))
	g.Expect(c.Reason).To(Equal(expectedReason))
}

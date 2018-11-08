package mysql

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	. "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	. "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestProvision(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}}
	h := &MockResourceHandler{}
	i := testInstance()

	// test: without ResourceClass definition - expected to: fail
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))

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
	g.Expect(rs).To(Equal(resultRequeue))

	// test: ResourceClass has no provisioner information
	mc.MockGet = func(args ...interface{}) error { return nil }
	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(result))

	// test: ResourceClass has RDS provisioner, but provisioning failed
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	handlers["test-provisioner"] = h
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp *MySQLInstance, cl client.Client) (corev1alpha1.Resource, error) {
		return nil, fmt.Errorf("test-provisioning-error")
	}

	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))

	// test: ResourceClass has RDS provisioner, no provisioning failures
	mc.MockGet = func(args ...interface{}) error {
		class := args[2].(*corev1alpha1.ResourceClass)
		class.Provisioner = "test-provisioner"
		return nil
	}
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp *MySQLInstance, cl client.Client) (corev1alpha1.Resource, error) {
		return &corev1alpha1.BasicResource{}, nil
	}
	r.bind = func(*MySQLInstance) (reconcile.Result, error) {
		return result, nil
	}

	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(result))
}

func TestBind(t *testing.T) {
	mc := &MockClient{}
	g := NewGomegaWithT(t)
	r := Reconciler{Client: mc, recorder: &MockRecorder{}}
	i := testInstance()
	h := &MockResourceHandler{}

	// test bind - handler is not found
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(result))
	c := i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))

	// test bind - error  retrieving resource instance
	i.Status.Provisioner = "test"
	handlers["test"] = h
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.Resource, error) {
		return nil, fmt.Errorf("test-error")
	}
	i.Spec.ResourceRef = &corev1.ObjectReference{
		Namespace: "foo",
		Name:      "bar",
	}
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
	c = i.Status.Condition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceInstance))

	// resource is not available
	br := corev1alpha1.NewBasicResource(nil, "", "", "not-available")
	h.MockFind = func(types.NamespacedName, client.Client) (corev1alpha1.Resource, error) {
		return br, nil
	}
	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
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
	g.Expect(rs).To(Equal(resultRequeue))
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
	g.Expect(rs).To(Equal(resultRequeue))
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
	g.Expect(rs).To(Equal(result))
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
	r := Reconciler{Client: mc, recorder: &MockRecorder{}}
	i := testInstance()
	h := &MockResourceHandler{}

	// test bind - handler is not found
	handlers["test"] = h
	mc.MockUpdate = func(...interface{}) error { return nil }
	rs, err := r._delete(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(result))
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
	g.Expect(rs).To(Equal(result))
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
	r := Reconciler{Client: mc, recorder: &MockRecorder{}}

	// reconciler function
	rfFlag := false
	rf := func(instance *MySQLInstance) (reconcile.Result, error) {
		rfFlag = true
		return result, nil
	}

	// failed to retrieve instance
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-instance"},
	}
	mc.MockGet = func(...interface{}) error {
		return fmt.Errorf("test-error")
	}
	rs, err := r.Reconcile(req)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(Equal("test-error"))

	// failed to retrieve instance (not found)
	mc.MockGet = func(...interface{}) error {
		return errors.NewNotFound(schema.GroupResource{Group: "foo", Resource: "bar"}, "test-instance")
	}
	rs, err = r.Reconcile(req)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())

	// reconcile deleted instance
	tm := v1.Now()
	mc.MockGet = func(args ...interface{}) error {
		i := args[2].(*MySQLInstance)
		i.DeletionTimestamp = &tm
		return nil
	}
	r.delete = rf
	rs, err = r.Reconcile(req)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(rfFlag).To(BeTrue())

	// add finalizer failed
	mc.MockGet = func(...interface{}) error {
		return nil
	}
	mc.MockUpdate = func(...interface{}) error {
		return fmt.Errorf("test-error")
	}
	r.delete = nil
	rs, err = r.Reconcile(req)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(Equal("test-error"))

	// provision
	mc.MockGet = func(...interface{}) error {
		return nil
	}
	mc.MockUpdate = func(...interface{}) error {
		return nil
	}
	rfFlag = false
	r.provision = rf
	rs, err = r.Reconcile(req)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(rfFlag).To(BeTrue())

	// bind
	mc.MockGet = func(args ...interface{}) error {
		i := args[2].(*MySQLInstance)
		i.Finalizers = append(i.Finalizers, finalizer)
		i.Spec.ResourceRef = &corev1.ObjectReference{}
		return nil
	}
	mc.MockUpdate = nil
	r.provision = nil
	rfFlag = false
	r.bind = rf
	rs, err = r.Reconcile(req)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(rfFlag).To(BeTrue())
}

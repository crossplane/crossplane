package mysql

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	mysqlv1alpha1 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	v1alpha12 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	. "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
	c := i.Status.GetCondition(corev1alpha1.Failed)
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
	c = i.Status.GetCondition(corev1alpha1.Failed)
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
	c = i.Status.GetCondition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceInstance))
	c = i.Status.GetCondition(corev1alpha1.Pending)
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
	mk.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test-error")
	})
	r.kubeclient = mk

	rs, err = r._bind(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
	c = i.Status.GetCondition(corev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorRetrievingResourceSecret))
}

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
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp *v1alpha12.MySQLInstance, cl client.Client) (corev1alpha1.Resource, error) {
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
	h.MockProvision = func(c *corev1alpha1.ResourceClass, sp *v1alpha12.MySQLInstance, cl client.Client) (corev1alpha1.Resource, error) {
		return &corev1alpha1.BasicResource{}, nil
	}
	r.bind = func(*mysqlv1alpha1.MySQLInstance) (reconcile.Result, error) {
		return result, nil
	}

	rs, err = r._provision(i)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(result))
}

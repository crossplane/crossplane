/*
Copyright 2018 The Conductor Authors.

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

package rds

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/aws"
	. "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws/rds"
	. "github.com/upbound/conductor/pkg/clients/aws/rds/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	. "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	. "k8s.io/client-go/testing"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace    = "default"
	providerName = "test-provider"
	instanceName = "test-instance"

	masterUserName = "testuser"
	engine         = "mysql"
	class          = "db.t2.small"
	size           = int64(10)
)

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      instanceName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
)

func init() {
	if err := aws.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func testProvider() *awsv1alpha1.Provider {
	return &awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
	}
}

func testResource() *RDSInstance {
	return &RDSInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: RDSInstanceSpec{
			MasterUsername: masterUserName,
			Engine:         engine,
			Class:          class,
			Size:           size,
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus) *RDSInstance {
	resource := &RDSInstance{}
	err := r.Get(ctx, key, resource)
	g.Expect(err).To(BeNil())
	g.Expect(resource.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(s))
	return resource
}

func TestSyncClusterGetError(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	testError := "test-resource-retrieve-error"

	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return nil, fmt.Errorf(testError)
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterStateCreating(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	called := false

	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateCreating),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetCreating()

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterStateFailed(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	called := false

	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateFailed),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, "resource is in failed state")

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterStateDeleting(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	called := false

	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateDeleting),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, fmt.Sprintf("unexpected resource status: %s", RDSInstanceStateDeleting))

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterGetSecretFailure(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateAvailable),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.SetFailed(errorSyncResource, fmt.Sprintf("secrets \"%s\" not found", tr.Name))

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterUpdateSecretFailure(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()
	ts := connectionSecret(tr, "testPassword")

	testError := "test-error-create-secret"
	kc := NewSimpleClientset(ts)
	kc.PrependReactor("update", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: kc,
	}

	called := false
	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateAvailable),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.SetFailed(errorSyncResource, testError)

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncCluster(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()
	ts := connectionSecret(tr, "testPassword")

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(ts),
	}

	called := false
	cl := &MockRDSClient{
		MockGetInstance: func(s string) (instance *rds.Instance, e error) {
			called = true
			return &rds.Instance{
				Status: string(RDSInstanceStateAvailable),
			}, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()

	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	cl := &MockRDSClient{}

	// test delete w/ reclaim policy
	tr.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetDeleting()

	rs, err := r._delete(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus)

	// test delete w/ delete policy
	tr.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	called := false
	cl.MockDeleteInstance = func(name string) (instance *rds.Instance, e error) {
		called = true
		return nil, nil
	}

	rs, err = r._delete(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// test delete w/ delete policy and delete error
	testError := "test-delete-error"
	called = false
	cl.MockDeleteInstance = func(name string) (instance *rds.Instance, e error) {
		called = true
		return nil, fmt.Errorf(testError)
	}
	expectedStatus.SetFailed(errorDeleteResource, testError)

	rs, err = r._delete(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	tk := NewSimpleClientset()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: tk,
	}

	called := false
	cl := &MockRDSClient{
		MockCreateInstance: func(s string, s2 string, spec *RDSInstanceSpec) (instance *rds.Instance, e error) {
			called = true
			return nil, nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetCreating()

	rs, err := r._create(testResource(), cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
	// assertSecret
	g.Expect(tk.Actions()).To(HaveLen(2))
	g.Expect(tk.Actions()[0].GetVerb()).To(Equal("get"))
	g.Expect(tk.Actions()[1].GetVerb()).To(Equal("create"))
	s, err := tk.CoreV1().Secrets(tr.Namespace).Get(tr.Name, metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s).NotTo(BeNil())
}

func TestCreateFail(t *testing.T) {
	g := NewGomegaWithT(t)
	tr := testResource()
	tk := NewSimpleClientset()
	cl := &MockRDSClient{}

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: tk,
	}

	// test apply secret error
	testError := "test-get-secret-error"
	tk.PrependReactor("get", "secrets", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateResource, testError)

	rs, err := r._create(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus)

	// test create resource error
	tr = testResource()
	r.kubeclient = NewSimpleClientset()
	called := false
	testError = "test-create-error"
	cl.MockCreateInstance = func(s string, s2 string, spec *RDSInstanceSpec) (instance *rds.Instance, e error) {
		called = true
		return nil, fmt.Errorf(testError)
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateResource, testError)

	rs, err = r._create(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestConnect(t *testing.T) {
	g := NewGomegaWithT(t)

	tp := testProvider()
	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(),
	}

	// provider status is not ready
	c, err := r._connect(tr)
	g.Expect(c).To(BeNil())
	g.Expect(err).To(And(Not(BeNil()), MatchError("provider is not ready")))

	// error getting aws config - secret is not found
	tp.Status.SetReady()
	r.Client = NewFakeClient(tp)
	c, err = r._connect(tr)
	g.Expect(c).To(BeNil())
	g.Expect(err).To(Not(BeNil()))
	t.Logf("e: %v", err)
}

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	// test connect error
	called := false
	testError := "test-connect-error"
	r.connect = func(instance *RDSInstance) (client rds.Client, e error) {
		called = true
		return nil, fmt.Errorf(testError)
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorResourceClient, testError)

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// test delete
	r.connect = func(instance *RDSInstance) (client rds.Client, e error) {
		t := metav1.Now()
		instance.DeletionTimestamp = &t
		return nil, nil
	}
	called = false
	r.delete = func(instance *RDSInstance, client rds.Client) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())

	// test create
	r.connect = func(instance *RDSInstance) (client rds.Client, e error) {
		return nil, nil
	}
	called = false
	r.delete = r._delete
	r.create = func(instance *RDSInstance, client rds.Client) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())

	// test sync
	r.connect = func(instance *RDSInstance) (client rds.Client, e error) {
		instance.Status.InstanceName = "foo"
		return nil, nil
	}
	called = false
	r.create = r._create
	r.sync = func(instance *RDSInstance, client rds.Client) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())

}

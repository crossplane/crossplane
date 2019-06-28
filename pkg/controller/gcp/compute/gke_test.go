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

package compute

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"google.golang.org/api/container/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	. "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	. "k8s.io/client-go/testing"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp"
	. "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/fake"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/gke"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace    = "default"
	providerName = "test-provider"
	clusterName  = "test-cluster"
)

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}

	masterAuth = &container.MasterAuth{
		Username:             "test-user",
		Password:             "test-pass",
		ClusterCaCertificate: base64.StdEncoding.EncodeToString([]byte("test-ca")),
		ClientCertificate:    base64.StdEncoding.EncodeToString([]byte("test-cert")),
		ClientKey:            base64.StdEncoding.EncodeToString([]byte("test-key")),
	}
)

func init() {
	_ = gcp.AddToScheme(scheme.Scheme)
}

func testCluster() *GKECluster {
	return &GKECluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: GKEClusterSpec{
			ResourceSpec: corev1alpha1.ResourceSpec{
				ProviderReference: &corev1.ObjectReference{Namespace: namespace, Name: providerName},
			},
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus) *GKECluster {
	rc := &GKECluster{}
	err := r.Get(ctx, key, rc)
	g.Expect(err).To(BeNil())
	g.Expect(cmp.Diff(s, rc.Status.ConditionedStatus, test.EquateConditions())).Should(BeZero())
	return rc
}

func TestSyncClusterGetError(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	testError := errors.New("test-cluster-retriever-error")

	cl := fake.NewGKEClient()
	cl.MockGetCluster = func(string, string) (*container.Cluster, error) {
		called = true
		return nil, testError
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(testError))

	rs, err := r._sync(tc, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncClusterNotReady(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false

	cl := fake.NewGKEClient()
	cl.MockGetCluster = func(string, string) (*container.Cluster, error) {
		called = true
		return &container.Cluster{
			Status: ClusterStateProvisioning,
		}, nil
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}

	rs, err := r._sync(tc, cl)
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: requeueOnWait}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSyncApplySecretError(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()

	testError := errors.New("test-error-create-secret")
	kc := NewSimpleClientset()
	kc.PrependReactor("create", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, testError
	})
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: kc,
	}

	called := false

	auth := masterAuth
	endpoint := "test-ep"

	cl := fake.NewGKEClient()
	cl.MockGetCluster = func(string, string) (*container.Cluster, error) {
		called = true
		return &container.Cluster{
			Status:     ClusterStateRunning,
			Endpoint:   endpoint,
			MasterAuth: auth,
		}, nil
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(testError))

	rs, err := r._sync(tc, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestSync(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false

	auth := masterAuth
	endpoint := "test-ep"

	cl := fake.NewGKEClient()
	cl.MockGetCluster = func(string, string) (*container.Cluster, error) {
		called = true
		return &container.Cluster{
			Status:     ClusterStateRunning,
			Endpoint:   endpoint,
			MasterAuth: auth,
		}, nil
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess())

	rs, err := r._sync(tc, cl)
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: requeueOnSucces}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestDeleteReclaimDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Finalizers = []string{finalizer}
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	cl := fake.NewGKEClient()
	cl.MockDeleteCluster = func(string, string) error {
		called = true
		return nil
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess())

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestDeleteReclaimRetain(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	tc.Finalizers = []string{finalizer}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	cl := fake.NewGKEClient()
	cl.MockDeleteCluster = func(string, string) error {
		called = true
		return nil
	}

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	// there should be no delete calls on gke client since policy is set to Retain
	g.Expect(called).To(BeFalse())

	// expected to have all conditions set to inactive
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess())

	assertResource(g, r, expectedStatus)
}

func TestDeleteFailed(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	tc.Finalizers = []string{finalizer}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	testError := errors.New("test-delete-error")

	called := false
	cl := fake.NewGKEClient()
	cl.MockDeleteCluster = func(string, string) error {
		called = true
		return testError
	}

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	// there should be no delete calls on gke client since policy is set to Retain
	g.Expect(called).To(BeTrue())

	// expected status
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileError(testError))

	assertResource(g, r, expectedStatus)
}

func TestReconcileObjectNotFound(t *testing.T) {
	g := NewGomegaWithT(t)

	r := &Reconciler{
		Client: NewFakeClient(),
	}
	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
}

func TestReconcileClientError(t *testing.T) {
	g := NewGomegaWithT(t)

	testError := errors.New("test-client-error")

	called := false

	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*GKECluster) (gke.Client, error) {
			called = true
			return nil, testError
		},
	}

	// expected to have a failed condition
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(testError))

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())

	assertResource(g, r, expectedStatus)
}

func TestReconcileDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	// test objects
	tc := testCluster()
	dt := metav1.Now()
	tc.DeletionTimestamp = &dt

	called := false

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*GKECluster) (gke.Client, error) {
			return nil, nil
		},
		delete: func(*GKECluster, gke.Client) (reconcile.Result, error) {
			called = true
			return result, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, corev1alpha1.ConditionedStatus{})
}

func TestReconcileCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	called := false

	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*GKECluster) (gke.Client, error) {
			return nil, nil
		},
		create: func(*GKECluster, gke.Client) (reconcile.Result, error) {
			called = true
			return resultRequeue, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())
}

func TestReconcileSync(t *testing.T) {
	g := NewGomegaWithT(t)

	called := false

	tc := testCluster()
	tc.Status.ClusterName = "test-status- cluster-name"
	tc.Finalizers = []string{finalizer}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*GKECluster) (gke.Client, error) {
			return nil, nil
		},
		sync: func(*GKECluster, gke.Client) (reconcile.Result, error) {
			called = true
			return resultRequeue, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())

	rc := assertResource(g, r, corev1alpha1.ConditionedStatus{})
	g.Expect(rc.Finalizers).To(HaveLen(1))
	g.Expect(rc.Finalizers).To(ContainElement(finalizer))
}

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

package compute

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/aws"
	. "github.com/upbound/conductor/pkg/apis/aws/compute/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws/eks"
	"github.com/upbound/conductor/pkg/clients/aws/eks/fake"
	corev1 "k8s.io/api/core/v1"
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
)

func init() {
	_ = aws.AddToScheme(scheme.Scheme)
}

func testCluster() *EKSCluster {
	return &EKSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: EKSClusterSpec{
			ProviderRef: corev1.LocalObjectReference{
				Name: providerName,
			},
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus) *EKSCluster {
	rc := &EKSCluster{}
	err := r.Get(ctx, key, rc)
	g.Expect(err).To(BeNil())
	g.Expect(rc.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(s))
	return rc
}

func TestSync(t *testing.T) {
	g := NewGomegaWithT(t)

	test := func(cl *fake.MockEKSClient, sec func(*eks.Cluster, *EKSCluster, eks.Client) error,
		rslt reconcile.Result, exp corev1alpha1.ConditionedStatus) {
		tc := testCluster()
		r := &Reconciler{
			Client:     NewFakeClient(tc),
			kubeclient: NewSimpleClientset(),
			secret:     sec,
		}

		rs, err := r._sync(tc, cl)
		g.Expect(rs).To(Equal(rslt))
		g.Expect(err).NotTo(HaveOccurred())
		assertResource(g, r, exp)
	}

	// error retrieving the cluster
	testError := "test-cluster-retriever-error"
	cl := &fake.MockEKSClient{
		MockGet: func(string) (*eks.Cluster, error) {
			return nil, fmt.Errorf(testError)
		},
	}
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncCluster, testError)
	test(cl, nil, resultRequeue, expectedStatus)

	// cluster is not ready
	cl.MockGet = func(string) (*eks.Cluster, error) {
		return &eks.Cluster{
			Status: ClusterStatusCreating,
		}, nil
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	test(cl, nil, resultRequeue, expectedStatus)

	// cluster is ready, but secret failed
	cl.MockGet = func(string) (*eks.Cluster, error) {
		return &eks.Cluster{
			Status: ClusterStatusActive,
		}, nil
	}
	testError = "test-create-secret-error"
	fSec := func(*eks.Cluster, *EKSCluster, eks.Client) error {
		return fmt.Errorf(testError)
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncCluster, testError)
	test(cl, fSec, resultRequeue, expectedStatus)

	// cluster is ready
	fSec = func(*eks.Cluster, *EKSCluster, eks.Client) error {
		return nil
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	test(cl, fSec, result, expectedStatus)
}

func TestSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()

	// connection token error
	testError := "test-connection-token-error"
	client := &fake.MockEKSClient{
		MockConnectionToken: func(string) (string, error) {
			return "", fmt.Errorf(testError)
		},
	}

	cluster := &eks.Cluster{
		Status:   ClusterStatusActive,
		Endpoint: "test-ep",
		CA:       "test-ca",
	}

	kc := NewSimpleClientset()
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: kc,
	}

	err := r._secret(cluster, tc, client)
	g.Expect(err).To(And(HaveOccurred(), MatchError(testError)))

	// test success
	client.MockConnectionToken = func(string) (string, error) { return "test-token", nil }
	err = r._secret(cluster, tc, client)

	g.Expect(err).NotTo(HaveOccurred())
	// validate secret
	secret, err := kc.CoreV1().Secrets(tc.Namespace).Get(tc.Name, metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	data := make(map[string][]byte)
	data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(cluster.Endpoint)
	data[corev1alpha1.ResourceCredentialsSecretCAKey] = []byte(cluster.CA)
	data[corev1alpha1.ResourceCredentialsToken] = []byte("test-token")
	secret.Data = data
	expSec := tc.ConnectionSecret()
	expSec.Data = data
	g.Expect(secret).To(Equal(expSec))

	// test update secret error
	testError = "test-update-secret-error"
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})

	err = r._secret(cluster, tc, client)
	g.Expect(err).To(And(HaveOccurred(), MatchError(testError)))
}

func TestDeleteReclaimDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Finalizers = []string{finalizer}
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	tc.Status.SetReady()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	cl := &fake.MockEKSClient{}
	cl.MockDelete = func(string) error {
		called = true
		return nil
	}

	// expected to have a cond condition set to inactive
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.UnsetAllConditions()

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// repeat the same for cluster in 'failing' condition
	reason := "test-reason"
	msg := "test-msg"
	tc.Status.SetFailed(reason, msg)

	// expected to have both ready and fail condition inactive
	expectedStatus.SetFailed(reason, msg)
	expectedStatus.UnsetAllConditions()

	rs, err = r._delete(tc, cl)
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
	tc.Status.SetReady()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	called := false
	cl := &fake.MockEKSClient{}
	cl.MockDelete = func(string) error {
		called = true
		return nil
	}

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	// there should be no delete calls on eks client since policy is set to Retain
	g.Expect(called).To(BeFalse())

	// expected to have all conditions set to inactive
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.UnsetAllConditions()

	assertResource(g, r, expectedStatus)
}

func TestDeleteFailed(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	tc.Finalizers = []string{finalizer}
	tc.Status.SetReady()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	testError := "test-delete-error"

	called := false
	cl := &fake.MockEKSClient{}
	cl.MockDelete = func(string) error {
		called = true
		return fmt.Errorf(testError)
	}

	rs, err := r._delete(tc, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	// there should be no delete calls on eks client since policy is set to Retain
	g.Expect(called).To(BeTrue())

	// expected status
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorDeleteCluster, testError)

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

	testError := "test-client-error"

	called := false

	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*EKSCluster) (eks.Client, error) {
			called = true
			return nil, fmt.Errorf(testError)
		},
	}

	// expected to have a failed condition
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorClusterClient, testError)

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
		connect: func(*EKSCluster) (eks.Client, error) {
			return nil, nil
		},
		delete: func(*EKSCluster, eks.Client) (reconcile.Result, error) {
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
		connect: func(*EKSCluster) (eks.Client, error) {
			return nil, nil
		},
		create: func(*EKSCluster, eks.Client) (reconcile.Result, error) {
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

func TestReconcileSync(t *testing.T) {
	g := NewGomegaWithT(t)

	called := false

	tc := testCluster()
	tc.Status.ClusterName = "test-status- cluster-name"
	tc.Finalizers = []string{finalizer}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*EKSCluster) (eks.Client, error) {
			return nil, nil
		},
		sync: func(*EKSCluster, eks.Client) (reconcile.Result, error) {
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

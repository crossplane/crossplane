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
	"fmt"
	"testing"

	"github.com/crossplaneio/crossplane/pkg/apis/aws"
	. "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks/fake"
	. "github.com/onsi/gomega"
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

func TestCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	test := func(cluster *EKSCluster, client eks.Client, expectedResult reconcile.Result, expectedStatus corev1alpha1.ConditionedStatus) *EKSCluster {
		r := &Reconciler{
			Client:     NewFakeClient(cluster),
			kubeclient: NewSimpleClientset(),
		}

		rs, err := r._create(cluster, client)
		g.Expect(rs).To(Equal(expectedResult))
		g.Expect(err).To(BeNil())
		return assertResource(g, r, expectedStatus)
	}

	// new cluster
	cluster := testCluster()
	cluster.ObjectMeta.UID = types.UID("test-uid")

	client := &fake.MockEKSClient{
		MockCreate: func(string, EKSClusterSpec) (*eks.Cluster, error) { return nil, nil },
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetCreating()

	reconciledCluster := test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(ContainElement(finalizer))
	g.Expect(reconciledCluster.Status.ClusterName).To(Equal(fmt.Sprintf("%s%s", clusterNamePrefix, cluster.UID)))
	g.Expect(reconciledCluster.State()).To(Equal(ClusterStatusCreating))

	// cluster create error - bad request
	cluster = testCluster()
	cluster.ObjectMeta.UID = types.UID("test-uid")
	client.MockCreate = func(string, EKSClusterSpec) (*eks.Cluster, error) {
		return nil, fmt.Errorf("InvalidParameterException")
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateCluster, "InvalidParameterException")

	reconciledCluster = test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())
	g.Expect(reconciledCluster.Status.ClusterName).To(BeEmpty())
	g.Expect(reconciledCluster.State()).To(BeEmpty())

	// cluster create error - other
	cluster = testCluster()
	cluster.ObjectMeta.UID = types.UID("test-uid")
	testError := "test-create-error"
	client.MockCreate = func(string, EKSClusterSpec) (*eks.Cluster, error) {
		return nil, fmt.Errorf(testError)
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateCluster, testError)

	reconciledCluster = test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())
	g.Expect(reconciledCluster.Status.ClusterName).To(BeEmpty())
	g.Expect(reconciledCluster.State()).To(BeEmpty())
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

func TestDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	test := func(cluster *EKSCluster, client eks.Client, expectedResult reconcile.Result, expectedStatus corev1alpha1.ConditionedStatus) *EKSCluster {
		r := &Reconciler{
			Client:     NewFakeClient(cluster),
			kubeclient: NewSimpleClientset(),
		}

		rs, err := r._delete(cluster, client)
		g.Expect(rs).To(Equal(expectedResult))
		g.Expect(err).To(BeNil())
		return assertResource(g, r, expectedStatus)
	}

	// reclaim - delete
	cluster := testCluster()
	cluster.Finalizers = []string{finalizer}
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	cluster.Status.SetReady()

	client := &fake.MockEKSClient{}
	client.MockDelete = func(string) error { return nil }

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.SetDeleting()

	reconciledCluster := test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())

	// reclaim - retain
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	cluster.Status.RemoveAllConditions()
	cluster.Status.SetReady()
	cluster.Finalizers = []string{finalizer}
	client.MockDelete = nil // should not be called

	reconciledCluster = test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())

	// reclaim - delete, delete error
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	cluster.Status.RemoveAllConditions()
	cluster.Status.SetReady()
	cluster.Finalizers = []string{finalizer}
	testError := "test-delete-error"
	client.MockDelete = func(string) error { return fmt.Errorf(testError) }
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.SetFailed(errorDeleteCluster, testError)

	reconciledCluster = test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(ContainElement(finalizer))
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

	assertResource(g, r, corev1alpha1.ConditionedStatus{})
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

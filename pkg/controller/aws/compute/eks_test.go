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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/ghodss/yaml"
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

	"github.com/crossplaneio/crossplane/pkg/apis/aws"
	. "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks/fake"
	"github.com/crossplaneio/crossplane/pkg/resource"
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
			ResourceSpec: corev1alpha1.ResourceSpec{
				ProviderReference: &corev1.ObjectReference{
					Name: providerName,
				},
			},
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus) *EKSCluster {
	rc := &EKSCluster{}
	err := r.Get(ctx, key, rc)
	g.Expect(err).To(BeNil())
	g.Expect(cmp.Diff(s, rc.Status.ConditionedStatus, test.EquateConditions())).Should(BeZero())
	return rc
}

func TestGenerateEksAuth(t *testing.T) {
	g := NewGomegaWithT(t)
	arnName := "test-arn"
	var expectRoles []MapRole
	var expectUsers []MapUser

	defaultMapRole := MapRole{
		RoleARN:  arnName,
		Username: "system:node:{{EC2PrivateDNSName}}",
		Groups:   []string{"system:bootstrappers", "system:nodes"},
	}

	exampleMapRole := MapRole{
		RoleARN:  "arn:aws:iam::000000000000:role/KubernetesAdmin",
		Username: "kubernetes-admin",
		Groups:   []string{"system:masters"},
	}

	exampleMapUser := MapUser{
		UserARN:  "arn:aws:iam::000000000000:user/Alice",
		Username: "alice",
		Groups:   []string{"system:masters"},
	}

	expectRoles = append(expectRoles, exampleMapRole)
	expectUsers = append(expectUsers, exampleMapUser)

	cluster := testCluster()
	cluster.Spec.MapRoles = expectRoles
	cluster.Spec.MapUsers = expectUsers

	// Default is included by so we don't add it to spec
	expectRoles = append(expectRoles, defaultMapRole)

	cm, err := generateAWSAuthConfigMap(cluster, arnName)
	g.Expect(err).To(BeNil())

	g.Expect(cm.Name).To(Equal("aws-auth"))
	g.Expect(cm.Namespace).To(Equal("kube-system"))

	var outputRoles []MapRole
	val := cm.Data["mapRoles"]
	err = yaml.Unmarshal([]byte(val), &outputRoles)
	g.Expect(err).To(BeNil())

	var outputUsers []MapUser
	val = cm.Data["mapUsers"]
	err = yaml.Unmarshal([]byte(val), &outputUsers)
	g.Expect(err).To(BeNil())

	g.Expect(outputRoles).To(Equal(expectRoles))
	g.Expect(outputUsers).To(Equal(expectUsers))
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
	expectedStatus.SetConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileSuccess())

	reconciledCluster := test(cluster, client, resultRequeue, expectedStatus)

	g.Expect(reconciledCluster.Status.ClusterName).To(Equal(fmt.Sprintf("%s%s", clusterNamePrefix, cluster.UID)))
	g.Expect(reconciledCluster.Status.State).To(Equal(ClusterStatusCreating))

	// cluster create error - bad request
	cluster = testCluster()
	cluster.ObjectMeta.UID = types.UID("test-uid")
	errorBadRequest := errors.New("InvalidParameterException")
	client.MockCreate = func(string, EKSClusterSpec) (*eks.Cluster, error) {
		return nil, errorBadRequest
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileError(errorBadRequest))

	reconciledCluster = test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())
	g.Expect(reconciledCluster.Status.ClusterName).To(BeEmpty())
	g.Expect(reconciledCluster.Status.State).To(BeEmpty())
	g.Expect(reconciledCluster.Status.CloudFormationStackID).To(BeEmpty())

	// cluster create error - other
	cluster = testCluster()
	cluster.ObjectMeta.UID = types.UID("test-uid")
	errorOther := errors.New("other")
	client.MockCreate = func(string, EKSClusterSpec) (*eks.Cluster, error) {
		return nil, errorOther
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileError(errorOther))

	reconciledCluster = test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())
	g.Expect(reconciledCluster.Status.ClusterName).To(BeEmpty())
	g.Expect(reconciledCluster.Status.State).To(BeEmpty())
	g.Expect(reconciledCluster.Status.CloudFormationStackID).To(BeEmpty())
}

func TestSync(t *testing.T) {
	g := NewGomegaWithT(t)
	fakeStackID := "fake-stack-id"

	test := func(tc *EKSCluster, cl *fake.MockEKSClient, sec func(*eks.Cluster, *EKSCluster, eks.Client) error, auth func(*eks.Cluster, *EKSCluster, eks.Client, string) error,
		rslt reconcile.Result, exp corev1alpha1.ConditionedStatus) *EKSCluster {
		r := &Reconciler{
			Client:     NewFakeClient(tc),
			kubeclient: NewSimpleClientset(),
			secret:     sec,
			awsauth:    auth,
		}

		rs, err := r._sync(tc, cl)
		g.Expect(rs).To(Equal(rslt))
		g.Expect(err).NotTo(HaveOccurred())
		return assertResource(g, r, exp)
	}

	fakeWorkerARN := "fake-worker-arn"
	mockClusterWorker := eks.ClusterWorkers{
		WorkerStackID: fakeStackID,
		WorkerARN:     fakeWorkerARN,
	}

	// error retrieving the cluster
	errorGet := errors.New("retrieving cluster")
	cl := &fake.MockEKSClient{
		MockGet: func(string) (*eks.Cluster, error) {
			return nil, errorGet
		},
		MockCreateWorkerNodes: func(string, EKSClusterSpec) (*eks.ClusterWorkers, error) { return &mockClusterWorker, nil },
	}

	cl.MockGetWorkerNodes = func(string) (*eks.ClusterWorkers, error) {
		return &eks.ClusterWorkers{
			WorkersStatus: cloudformation.StackStatusCreateInProgress,
			WorkerReason:  "",
			WorkerStackID: fakeStackID}, nil
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(errorGet))
	tc := testCluster()
	test(tc, cl, nil, nil, resultRequeue, expectedStatus)

	// cluster is not ready
	cl.MockGet = func(string) (*eks.Cluster, error) {
		return &eks.Cluster{
			Status: ClusterStatusCreating,
		}, nil
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	tc = testCluster()
	test(tc, cl, nil, nil, resultRequeue, expectedStatus)

	// cluster is ready, but lets create workers that error
	cl.MockGet = func(string) (*eks.Cluster, error) {
		return &eks.Cluster{
			Status: ClusterStatusActive,
		}, nil
	}

	errorCreateNodes := errors.New("create nodes")
	cl.MockCreateWorkerNodes = func(string, EKSClusterSpec) (*eks.ClusterWorkers, error) {
		return nil, errorCreateNodes
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(errorCreateNodes))
	tc = testCluster()
	reconciledCluster := test(tc, cl, nil, nil, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Status.CloudFormationStackID).To(BeEmpty())

	// cluster is ready, lets create workers
	cl.MockGet = func(string) (*eks.Cluster, error) {
		return &eks.Cluster{
			Status: ClusterStatusActive,
		}, nil
	}

	cl.MockCreateWorkerNodes = func(string, EKSClusterSpec) (*eks.ClusterWorkers, error) {
		return &eks.ClusterWorkers{WorkerStackID: fakeStackID}, nil
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileSuccess())
	tc = testCluster()
	reconciledCluster = test(tc, cl, nil, nil, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Status.CloudFormationStackID).To(Equal(fakeStackID))

	// cluster is ready, but auth sync failed
	cl.MockGetWorkerNodes = func(string) (*eks.ClusterWorkers, error) {
		return &eks.ClusterWorkers{
			WorkersStatus: cloudformation.StackStatusCreateComplete,
			WorkerReason:  "",
			WorkerStackID: fakeStackID,
			WorkerARN:     fakeWorkerARN,
		}, nil
	}

	errorAuth := errors.New("auth")
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(errors.Wrap(errorAuth, "failed to set auth map on eks")))
	tc = testCluster()
	tc.Status.CloudFormationStackID = fakeStackID
	auth := func(*eks.Cluster, *EKSCluster, eks.Client, string) error {
		return errorAuth

	}
	test(tc, cl, nil, auth, resultRequeue, expectedStatus)

	// cluster is ready, but secret failed
	cl.MockGetWorkerNodes = func(string) (*eks.ClusterWorkers, error) {
		return &eks.ClusterWorkers{
			WorkersStatus: cloudformation.StackStatusCreateComplete,
			WorkerReason:  "",
			WorkerStackID: fakeStackID,
			WorkerARN:     fakeWorkerARN,
		}, nil
	}

	auth = func(*eks.Cluster, *EKSCluster, eks.Client, string) error {
		return nil
	}

	errorSecret := errors.New("secret")
	fSec := func(*eks.Cluster, *EKSCluster, eks.Client) error {
		return errorSecret
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.ReconcileError(errorSecret))
	tc = testCluster()
	tc.Status.CloudFormationStackID = fakeStackID
	test(tc, cl, fSec, auth, resultRequeue, expectedStatus)

	// cluster is ready
	fSec = func(*eks.Cluster, *EKSCluster, eks.Client) error {
		return nil
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess())
	tc = testCluster()
	tc.Status.CloudFormationStackID = fakeStackID
	test(tc, cl, fSec, auth, result, expectedStatus)
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
	clusterCA := []byte("test-ca")
	cluster := &eks.Cluster{
		Status:   ClusterStatusActive,
		Endpoint: "test-ep",
		CA:       base64.StdEncoding.EncodeToString(clusterCA),
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
	secret, err := kc.CoreV1().Secrets(tc.GetNamespace()).Get(tc.GetWriteConnectionSecretToReference().Name, metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	data := make(map[string][]byte)
	data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(cluster.Endpoint)
	data[corev1alpha1.ResourceCredentialsSecretCAKey] = clusterCA
	data[corev1alpha1.ResourceCredentialsTokenKey] = []byte("test-token")
	secret.Data = data
	expSec := resource.ConnectionSecretFor(tc, EKSClusterGroupVersionKind)
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
	cluster.Status.CloudFormationStackID = "fake-stack-id"
	cluster.Status.SetConditions(corev1alpha1.Available())

	client := &fake.MockEKSClient{}
	client.MockDelete = func(string) error { return nil }
	client.MockDeleteWorkerNodes = func(string) error { return nil }

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess())

	reconciledCluster := test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())

	// reclaim - retain
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	cluster.Status.SetConditions(corev1alpha1.Available())
	cluster.Finalizers = []string{finalizer}
	client.MockDelete = nil // should not be called

	reconciledCluster = test(cluster, client, result, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(BeEmpty())

	// reclaim - delete, delete error
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	cluster.Status.SetConditions(corev1alpha1.Available())
	cluster.Finalizers = []string{finalizer}
	errorDelete := errors.New("test-delete-error")
	client.MockDelete = func(string) error { return errorDelete }
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(
		corev1alpha1.Deleting(),
		corev1alpha1.ReconcileError(errors.Wrap(errorDelete, "Master Delete Error")),
	)

	reconciledCluster = test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(ContainElement(finalizer))

	// reclaim - delete, delete error with worker
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	cluster.Status.SetConditions(corev1alpha1.Available())
	cluster.Finalizers = []string{finalizer}
	testErrorWorker := errors.New("test-delete-error-worker")
	client.MockDelete = func(string) error { return nil }
	client.MockDeleteWorkerNodes = func(string) error { return testErrorWorker }
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(
		corev1alpha1.Deleting(),
		corev1alpha1.ReconcileError(errors.Wrap(testErrorWorker, "Worker Delete Error")),
	)

	reconciledCluster = test(cluster, client, resultRequeue, expectedStatus)
	g.Expect(reconciledCluster.Finalizers).To(ContainElement(finalizer))

	// reclaim - delete, delete error in cluster and cluster workers
	cluster.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	cluster.Status.SetConditions(corev1alpha1.Available())
	cluster.Finalizers = []string{finalizer}
	client.MockDelete = func(string) error { return nil }
	client.MockDelete = func(string) error { return errorDelete }
	client.MockDeleteWorkerNodes = func(string) error { return testErrorWorker }
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetConditions(
		corev1alpha1.Deleting(),
		corev1alpha1.ReconcileError(errors.New("Master Delete Error: test-delete-error, Worker Delete Error: test-delete-error-worker")),
	)

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

	testError := errors.New("test-client-error")

	called := false

	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*EKSCluster) (eks.Client, error) {
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

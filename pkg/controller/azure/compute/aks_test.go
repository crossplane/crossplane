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
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"

	apisazure "github.com/crossplaneio/crossplane/pkg/apis/azure"
	computeazurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/fake"
	. "github.com/crossplaneio/crossplane/pkg/controller/fake"
	"github.com/crossplaneio/crossplane/pkg/util"
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
	// test namespace for test artifacts
	namespace = "default"
	// test provider name for aks cluster
	providerName = "test-provider"
	// test cluster name
	clusterName = "test-cluster"
	// test credentials
	credentials = `
{
 "clientId": "test",
 "clientSecret": "test",
 "subscriptionId": "test",
 "tenantId": "test",
 "activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
 "resourceManagerEndpointUrl": "https://management.azure.com/",
 "activeDirectoryGraphResourceId": "https://graph.windows.net/",
 "sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
 "galleryEndpointUrl": "https://gallery.azure.com/",
 "managementEndpointUrl": "https://management.core.windows.net/"
}
`
)

var (
	// test key all tests
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	// test request for all tests
	request = reconcile.Request{
		NamespacedName: key,
	}
)

// init load schema for azure types
func init() {
	_ = apisazure.AddToScheme(scheme.Scheme)
}

// testProvider
func testProvider() *azurev1alpha1.Provider {
	return &azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
	}
}

// testClusters
func testCluster() *computeazurev1alpha1.AKSCluster {
	return &computeazurev1alpha1.AKSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: computeazurev1alpha1.AKSClusterSpec{
			ProviderRef: corev1.LocalObjectReference{
				Name: providerName,
			},
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus, testName string) *computeazurev1alpha1.AKSCluster {
	rc := &computeazurev1alpha1.AKSCluster{}
	err := r.Get(ctx, key, rc)
	g.Expect(err).To(BeNil(), testName)
	g.Expect(rc.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(s), testName)
	return rc
}

func TestAdd(t *testing.T) {

}

// --------------------------------------------------------------------------------------------------------------------
// Reconcile Test Group
// --------------------------------------------------------------------------------------------------------------------
func TestReconcile_ObjectNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	r := &Reconciler{
		Client: NewFakeClient(),
	}
	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).To(BeNil())
}

func TestReconcile_ClientError(t *testing.T) {
	g := NewGomegaWithT(t)
	testError := "test-client-error"
	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*computeazurev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
			return nil, fmt.Errorf(testError)
		},
	}
	// expected to have a failed condition
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(reasonCreateClusterClientFailure, testError)

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	assertResource(g, r, expectedStatus, t.Name())
}

func TestReconcile_Delete(t *testing.T) {
	g := NewGomegaWithT(t)
	// test objects
	tc := testCluster()
	dt := metav1.Now()
	tc.DeletionTimestamp = &dt
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*computeazurev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
			return nil, nil
		},
		delete: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error) {
			return resultDone, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).To(BeNil())
	assertResource(g, r, corev1alpha1.ConditionedStatus{}, t.Name())
}

func TestReconcile_WaitForRunningOperation(t *testing.T) {
	g := NewGomegaWithT(t)

	// test future
	f, err := fake.NewMockFutureFromResponseValues("foo.bar", http.MethodPut, http.StatusOK)
	g.Expect(err).NotTo(HaveOccurred())

	tc := testCluster()
	tc.Status.RunningOperation = f

	result := reconcile.Result{RequeueAfter: 5 * time.Minute}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*computeazurev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
			return nil, nil
		},
		wait: func(cluster *computeazurev1alpha1.AKSCluster, api azure.AKSClientsetAPI) (i reconcile.Result, e error) {
			return result, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
	assertResource(g, r, corev1alpha1.ConditionedStatus{}, t.Name())
}

func TestReconcile_Create(t *testing.T) {
	g := NewGomegaWithT(t)
	// reconciled cluster
	var rc *computeazurev1alpha1.AKSCluster
	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*computeazurev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
			return nil, nil
		},
		create: func(cluster *computeazurev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (reconcile.Result, error) {
			rc = cluster
			return resultRequeue, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	// assert reconciled cluster
	g.Expect(rc.Status.ConditionedStatus.Conditions).Should(BeEmpty())
	g.Expect(rc.Finalizers).To(HaveLen(1))
	g.Expect(rc.Finalizers).To(ContainElement(finalizer))
}

func TestReconcile_Sync(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "test-status- cluster-name"
	tc.Finalizers = []string{finalizer}

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		connect: func(*computeazurev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
			return nil, nil
		},
		sync: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error) {
			return resultRequeue, nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	rc := assertResource(g, r, corev1alpha1.ConditionedStatus{}, t.Name())
	g.Expect(rc.Finalizers).To(HaveLen(1))
	g.Expect(rc.Finalizers).To(ContainElement(finalizer))
}

// --------------------------------------------------------------------------------------------------------------------
// Connect Tests
// --------------------------------------------------------------------------------------------------------------------
func TestConnect_ProviderNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	r := Reconciler{
		Client: NewFakeClient(),
	}
	_, err := r._connect(tc)
	g.Expect(err).To(HaveOccurred())
}

func TestConnect_ProviderInvalid(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tp := testProvider()
	r := Reconciler{
		Client:     NewFakeClient(tc, tp),
		kubeclient: NewSimpleClientset(),
	}
	_, err := r._connect(tc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("provider status is invalid"))
}

func TestConnect_ProviderSecretRetrieveError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tp := testProvider()
	tp.Status.SetReady()
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test-secret-error")
	})
	r := Reconciler{
		Client:     NewFakeClient(tc, tp),
		kubeclient: kc,
	}
	_, err := r._connect(tc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("test-secret-error"))
}

func TestConnect_InvalidCreds(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tp := testProvider()
	tp.Status.SetReady()
	tp.Spec.Secret = corev1.SecretKeySelector{
		Key: "creds-key",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "creds-secret",
		},
	}
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, &corev1.Secret{
			Data: map[string][]byte{"creds-key": []byte("data")},
		}, nil
	})
	r := Reconciler{
		Client:     NewFakeClient(tc, tp),
		kubeclient: kc,
	}
	_, err := r._connect(tc)
	g.Expect(err).To(HaveOccurred())
}

func TestConnect_Valid(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tp := testProvider()
	tp.Status.SetReady()
	tp.Spec.Secret = corev1.SecretKeySelector{
		Key: "creds-key",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "creds-secret",
		},
	}
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, &corev1.Secret{
			Data: map[string][]byte{"creds-key": []byte(credentials)},
		}, nil
	})
	r := Reconciler{
		Client:                 NewFakeClient(tc, tp),
		kubeclient:             kc,
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(fake.NewFakeAKSClientset(), nil),
	}
	client, err := r._connect(tc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

// --------------------------------------------------------------------------------------------------------------------
// Create Tests
// --------------------------------------------------------------------------------------------------------------------
func TestCreate_ClusterExists(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateCreating, nil)

	expectedCondition := corev1alpha1.ConditionedStatus{}
	expectedCondition.SetCreating()
	expectedCondition.UnsetAllConditions()
	expectedCondition.SetFailed(reasonCreateClusterFailure, "")

	r := &Reconciler{
		Client:   NewFakeClient(tc),
		recorder: NewNilEventRecorder(),
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))

	rc := assertResource(g, r, tc.Status.ConditionedStatus, t.Name())
	g.Expect(rc.Status.ClusterName).NotTo(BeEmpty())
}

func TestCreate_GetClusterError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()

	testError := "error-getting-cluster"
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fmt.Errorf(testError))

	expectedCondition := corev1alpha1.ConditionedStatus{}
	expectedCondition.SetCreating()
	expectedCondition.UnsetAllConditions()
	expectedCondition.SetFailed(reasonCreateClusterFailure, testError)

	r := &Reconciler{
		Client: NewFakeClient(tc),
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
	assertResource(g, r, expectedCondition, t.Name())
}

func TestCreate_CreateAppError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fake.NotFoundError)

	r := Reconciler{
		Client: NewFakeClient(tc),
		createApp: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (string, string, error) {
			return "", "", fake.BadRequestError
		},
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
}

func TestCreate_BadRequest(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fake.NotFoundError)
	ac.MockCreateCluster = fake.NewMockCreateClusterFunction(nil, fake.BadRequestError)

	r := Reconciler{
		Client: NewFakeClient(tc),
		createApp: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (string, string, error) {
			return "test-id", "test-password", nil
		},
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultDone))
}

func TestCreate_ServerError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fake.NotFoundError)
	ac.MockCreateCluster = fake.NewMockCreateClusterFunction(nil, fake.InternalServerError)

	r := Reconciler{
		Client: NewFakeClient(tc),
		createApp: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (string, string, error) {
			return "test-id", "test-password", nil
		},
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))
}

func TestCreate_Success(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.UID = types.UID("test-uid")

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fake.NotFoundError)
	ac.MockCreateCluster = fake.NewMockCreateClusterFunction(&computeazurev1alpha1.AKSClusterFuture{}, nil)

	r := Reconciler{
		Client: NewFakeClient(tc),
		createApp: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (string, string, error) {
			return "test-id", "test-password", nil
		},
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultDone))

	// assert reconciled cluster
	rc := &computeazurev1alpha1.AKSCluster{}
	g.Expect(r.Get(ctx, key, rc)).To(Succeed())

	g.Expect(rc.Status.ClusterName).To(Equal("aks-test-uid"))
	g.Expect(rc.Status.IsCreating()).To(BeTrue())
	g.Expect(rc.Status.State).To(Equal(computeazurev1alpha1.ClusterStateCreating))
}

// --------------------------------------------------------------------------------------------------------------------
// Create App Tests
// --------------------------------------------------------------------------------------------------------------------
func TestCreateApp_DeleteExistingFailure(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Status.ApplicationObjectID = "test-ad-app-id"

	testError := "test-delete-failure"
	ac := fake.NewFakeAKSClientset()
	ac.MockDeleteApplication = func(ctx context.Context, objectID string) error {
		return fmt.Errorf(testError)
	}

	r := Reconciler{}
	_, _, err := r._createApp(tc, ac)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(testError))
}

func TestCreateApp_SecretError(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.DNSNamePrefix = "test.dns"
	tc.Spec.Location = "test location"

	testError := "test-secret-failure"
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})

	ac := fake.NewFakeAKSClientset()

	r := Reconciler{
		kubeclient: kc,
	}
	_, _, err := r._createApp(tc, ac)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(testError))
}

func TestCreateApp_CreateAppError(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.DNSNamePrefix = "test.dns"
	tc.Spec.Location = "test location"

	testError := "test-create-application-failure"
	ac := fake.NewFakeAKSClientset()
	ac.MockCreateApplication = fake.NewMockCreateApplicationFunction(nil, fmt.Errorf(testError))

	r := Reconciler{
		kubeclient: NewSimpleClientset(),
	}
	_, _, err := r._createApp(tc, ac)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(testError))
}

func TestCreateApp_CreateSPError(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.DNSNamePrefix = "test.dns"
	tc.Spec.Location = "test location"

	testError := "test-create-sp-failure"
	ac := fake.NewFakeAKSClientset()
	ac.MockCreateApplication = fake.NewMockCreateApplicationFunction(&graphrbac.Application{AppID: util.String("test-app-id")}, nil)
	ac.MockCreateServicePrincipal = func(ctx context.Context, appID string) (principal *graphrbac.ServicePrincipal, e error) {
		g.Expect(appID).To(Equal("test-app-id"))
		return nil, fmt.Errorf(testError)
	}

	r := Reconciler{
		kubeclient: NewSimpleClientset(),
	}
	_, _, err := r._createApp(tc, ac)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(testError))
}

func TestCreateApp_Success(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Spec.DNSNamePrefix = "test.dns"
	tc.Spec.Location = "test location"

	app := &graphrbac.Application{
		AppID:    util.String("test-app-id"),
		ObjectID: util.String("test-object-id"),
	}

	ac := fake.NewFakeAKSClientset()
	ac.MockCreateApplication = fake.NewMockCreateApplicationFunction(app, nil)
	ac.MockCreateServicePrincipal = func(ctx context.Context, appID string) (principal *graphrbac.ServicePrincipal, e error) {
		return nil, nil
	}

	r := Reconciler{
		kubeclient: NewSimpleClientset(),
	}
	a, p, err := r._createApp(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(a).To(Equal(*app.AppID))
	g.Expect(tc.Status.ApplicationObjectID).To(Equal(*app.ObjectID))
	g.Expect(p).NotTo(BeEmpty())
}

// --------------------------------------------------------------------------------------------------------------------
// Sync Tests
// --------------------------------------------------------------------------------------------------------------------
func TestSync_FailedToGetAKS(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to retrieve aks cluster"
	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunction(fake.BasicManagedCluster, fmt.Errorf(testError))
	r := &Reconciler{
		Client: NewFakeClient(tc),
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncFailCluster, testError)

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_ClusterStateCreating(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateCreating, nil)

	r := &Reconciler{
		Client:   NewFakeClient(tc),
		recorder: NewNilEventRecorder(),
	}
	expectedStatus := tc.Status.ConditionedStatus

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: syncWaitStatusCreating}))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_ClusterStateFailed(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateFailed, nil)

	r := &Reconciler{
		Client:   NewFakeClient(tc),
		recorder: NewNilEventRecorder(),
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.SetFailed(errorSyncUnexpectedClusterState, "failed")

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).NotTo(HaveOccurred())
	tc = assertResource(g, r, expectedStatus, t.Name())

	// repeat with failed condition - no change
	rs, err = r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_ClusterStateUnexpected(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState("Foo-Bar", nil)

	r := &Reconciler{
		Client:   NewFakeClient(tc),
		recorder: NewNilEventRecorder(),
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.SetFailed(errorSyncFailCluster, "unexpected cluster status: Foo-Bar")

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: syncWaitStatusUnexpected}))
	g.Expect(err).NotTo(HaveOccurred())
	tc = assertResource(g, r, expectedStatus, t.Name())

	// repeat with failed condition - no change
	rs, err = r._sync(tc, ac)
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: syncWaitStatusUnexpected}))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_FailedToRetrieveAKSCreds(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateSucceeded, nil)

	testError := "failed to retrieve aks creds"
	r := &Reconciler{
		Client: NewFakeClient(tc),
		secret: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (*corev1.Secret, error) {
			return nil, fmt.Errorf(testError)
		},
	}

	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncFailCluster, testError)

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_FailedToCreateConnectionSecret(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateSucceeded, nil)

	testError := "failed to create connection secret"
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: kc,
		secret: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (*corev1.Secret, error) {
			return &corev1.Secret{}, nil
		},
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncFailCluster, testError)

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_Success(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"foo": []byte("YmFyCg==")},
	}

	ac := fake.NewFakeAKSClientset()
	ac.MockGetCluster = fake.NewMockGetClusterFunctionWithState(computeazurev1alpha1.ClusterStateSucceeded, nil)

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		secret: func(*computeazurev1alpha1.AKSCluster, azure.AKSClientsetAPI) (*corev1.Secret, error) {
			return testSecret, nil
		},
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetReady()

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())

	// validate secret
	data, err := util.SecretData(r.kubeclient, "default", corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test-secret",
		},
		Key: "foo",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).NotTo(BeEmpty())
}

// --------------------------------------------------------------------------------------------------------------------
// Delete Tests
// --------------------------------------------------------------------------------------------------------------------
func TestDelete_ReclaimDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Finalizers = []string{finalizer}
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	tc.Status.SetReady()

	// Test delete error
	testError := "test-delete-error"
	ac := fake.NewFakeAKSClientset()
	ac.MockDelete = func(ctx context.Context, group, name, appID string) error { return fmt.Errorf(testError) }

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(reasonDeleteClusterFailure, testError)

	rs, err := r._delete(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())
	assertResource(g, r, expectedStatus, "error")

	// Test delete successful
	ac.MockDelete = func(ctx context.Context, group, name, appID string) error { return nil }
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetDeleting()

	rs, err = r._delete(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).To(BeNil())
	assertResource(g, r, expectedStatus, "ready")

	// repeat the same test for cluster in 'failing' condition
	reason := "test-reason"
	msg := "test-msg"
	tc.Status.SetFailed(reason, msg)

	// expected to have both ready and fail condition inactive
	expectedStatus.SetFailed(reason, msg)
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetDeleting()

	rs, err = r._delete(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).To(BeNil())
	assertResource(g, r, expectedStatus, t.Name()+"failed")
}

func TestDelete_ReclaimRetain(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	tc.Finalizers = []string{finalizer}
	tc.Status.SetReady()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	// expected to have all conditions set to inactive
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetDeleting()

	rs, err := r._delete(tc, fake.NewFakeAKSClientset())
	g.Expect(rs).To(Equal(resultDone))
	g.Expect(err).To(BeNil())

	assertResource(g, r, expectedStatus, t.Name())
}

// --------------------------------------------------------------------------------------------------------------------
// Secret Tests
// --------------------------------------------------------------------------------------------------------------------
func TestSecret_FailedListCredentials(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to list cluster kubeconfig creds"
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", nil, fmt.Errorf(testError))

	r := &Reconciler{}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(testError))
}

func TestSecret_FailedToLoadKubeconfig(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()

	data := []byte("foo")
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", data, nil)

	r := &Reconciler{
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(ac, nil),
	}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

const kubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: Y2VydGlmaWNhdGUtYXV0aG9yaXR5LWRhdGEK
    server: https://cluster-name:443
  name: cluster-name
contexts:
- context:
    cluster: cluster-name
    user: cluster-user
  name: cluster-context
current-context: cluster-context
preferences: {}
users:
- name: cluster-user
  user:
    client-certificate-data: Y2xpZW50LWNlcnRpZmljYXRlLWRhdGEK
    client-key-data: Y2xpZW50LWtleS1kYXRhCg==
    token: dG9rZW4K`

func TestSecret_KubeconfigContextNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "test-cluster-name"

	data := []byte(kubeconfig)
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", data, nil)

	r := &Reconciler{
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(ac, nil),
	}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestSecret_KubeconfigClusterNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(strings.Replace(kubeconfig, "name: cluster-name", "name: foo", -1))
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", data, nil)

	r := &Reconciler{
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(ac, nil),
	}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestSecret_KubeconfigAuthInfoNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(strings.Replace(kubeconfig, "name: cluster-user", "name: foo", -1))
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", data, nil)

	r := &Reconciler{
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(ac, nil),
	}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestSecret_Success(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(kubeconfig)
	ac := fake.NewFakeAKSClientset()
	ac.MockListCredentials = fake.NewMockListCredentialsFunction("", data, nil)

	r := &Reconciler{
		AKSClientsetFactoryAPI: fake.NewMockAKSClientsetFactory(ac, nil),
	}
	s, err := r._secret(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s).NotTo(BeNil())

	g.Expect(s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]).To(Equal([]byte("https://cluster-name:443")))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretCAKey]))).To(Equal("certificate-authority-data"))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey]))).To(Equal("client-certificate-data"))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey]))).To(Equal("client-key-data"))
}

// --------------------------------------------------------------------------------------------------------------------
// Wait Tests
// --------------------------------------------------------------------------------------------------------------------

func TestWait_DoneError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "test-wait-error"
	ac := fake.NewFakeAKSClientset()
	ac.MockDoneWithContext = func(ctx context.Context, future *computeazurev1alpha1.AKSClusterFuture) (bool, error) {
		return false, fmt.Errorf(testError)
	}

	r := &Reconciler{
		Client: NewFakeClient(tc),
	}

	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(reasonWaitFailed, testError)

	rs, err := r._wait(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(resultRequeue))

	assertResource(g, r, expectedStatus, t.Name())
}

func TestWait_Done(t *testing.T) {
	g := NewGomegaWithT(t)

	// test function for different status codes
	tf := func(statusCode int) {
		f, err := fake.NewMockFutureFromResponseValues("foo.bar", http.MethodPut, statusCode)
		g.Expect(err).NotTo(HaveOccurred())

		tc := testCluster()
		tc.Status.SetCreating()
		tc.Status.RunningOperation = f

		ac := fake.NewFakeAKSClientset()
		ac.MockDoneWithContext = func(ctx context.Context, future *computeazurev1alpha1.AKSClusterFuture) (bool, error) {
			return true, nil
		}
		ac.MockGetResult = func(future *computeazurev1alpha1.AKSClusterFuture) (response *http.Response, e error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		}

		r := &Reconciler{
			Client: NewFakeClient(tc),
		}

		expectedStatus := tc.Status.ConditionedStatus

		rs, err := r._wait(tc, ac)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(rs).To(Equal(resultDone))

		rc := assertResource(g, r, expectedStatus, fmt.Sprintf("%s, code: %s", t.Name(), http.StatusText(statusCode)))
		g.Expect(rc.Status.RunningOperation).To(BeNil())
	}

	tf(http.StatusOK)
	tf(http.StatusCreated)
}

func TestWait_Done_GetResultError(t *testing.T) {
	g := NewGomegaWithT(t)

	f, err := fake.NewMockFutureFromResponseValues("foo.bar", http.MethodPut, http.StatusOK)
	g.Expect(err).NotTo(HaveOccurred())

	tc := testCluster()
	tc.Status.SetCreating()
	tc.Status.RunningOperation = f

	testError := "test-get-result-error"

	ac := fake.NewFakeAKSClientset()
	ac.MockDoneWithContext = func(ctx context.Context, future *computeazurev1alpha1.AKSClusterFuture) (bool, error) {
		return true, nil
	}
	ac.MockGetResult = func(future *computeazurev1alpha1.AKSClusterFuture) (response *http.Response, e error) {
		return nil, fmt.Errorf(testError)
	}

	r := &Reconciler{
		recorder: NewNilEventRecorder(),
	}

	rs, err := r._wait(tc, ac)
	g.Expect(err).To(MatchError(testError))
	g.Expect(rs).To(Equal(resultDone))
}

func TestWait_Done_ResultNotOk(t *testing.T) {
	g := NewGomegaWithT(t)

	f, err := fake.NewMockFutureFromResponseValues("foo.bar", http.MethodPut, http.StatusOK)
	g.Expect(err).NotTo(HaveOccurred())

	tc := testCluster()
	tc.Status.SetCreating()
	tc.Status.RunningOperation = f

	ac := fake.NewFakeAKSClientset()
	ac.MockDoneWithContext = func(ctx context.Context, future *computeazurev1alpha1.AKSClusterFuture) (bool, error) {
		return true, nil
	}
	ac.MockGetResult = func(future *computeazurev1alpha1.AKSClusterFuture) (response *http.Response, e error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: ioutil.NopCloser(strings.NewReader("test-body"))}, nil
	}

	r := &Reconciler{
		Client:   NewFakeClient(tc),
		recorder: NewNilEventRecorder(),
	}

	rs, err := r._wait(tc, ac)
	g.Expect(rs).To(Equal(resultDone))
}

func TestWait_NotReady(t *testing.T) {
	g := NewGomegaWithT(t)

	f, err := fake.NewMockFutureFromResponseValues("foo.bar", http.MethodPut, http.StatusOK)
	g.Expect(err).NotTo(HaveOccurred())

	tc := testCluster()
	tc.Status.SetCreating()
	tc.Status.RunningOperation = f

	ac := fake.NewFakeAKSClientset()
	ac.MockDoneWithContext = func(ctx context.Context, future *computeazurev1alpha1.AKSClusterFuture) (bool, error) {
		return false, nil
	}

	r := &Reconciler{
		recorder: NewNilEventRecorder(),
	}

	rs, err := r._wait(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(reconcile.Result{RequeueAfter: waitDelay}))
}

// --------------------------------------------------------------------------------------------------------------------

func TestApplicationURL(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(applicationURL("dns", "location")).To(MatchRegexp("https://.*.dns.location.cloudapp.crossplane.io"))
	g.Expect(applicationURL("dns", "location with space")).To(MatchRegexp("https://.*.dns.locationwithspace.cloudapp.crossplane.io"))
}

func TestApplicationSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	data := []byte("test-data")
	s := applicationSecret(tc, data)

	g.Expect(s).To(Equal(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tc.Namespace,
			Name:            tc.Name + "-service-principal",
			OwnerReferences: []metav1.OwnerReference{tc.OwnerReference()},
		},
		Data: map[string][]byte{
			ApplicationSecretKey: data,
		},
	}))
}

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
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/crossplaneio/crossplane/pkg/apis/azure"
	. "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/aks"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/aks/fake"
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
	namespace    = "default"
	providerName = "test-provider"
	clusterName  = "test-cluster"

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
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
)

func init() {
	_ = azure.AddToScheme(scheme.Scheme)
}

func testProvider() *azurev1alpha1.Provider {
	return &azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
	}
}

func testCluster() *AKSCluster {
	return &AKSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: AKSClusterSpec{
			ProviderRef: corev1.LocalObjectReference{
				Name: providerName,
			},
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus, testName string) *AKSCluster {
	rc := &AKSCluster{}
	err := r.Get(ctx, key, rc)
	g.Expect(err).To(BeNil(), testName)
	g.Expect(rc.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(s), testName)
	return rc
}

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
		Client:     NewFakeClient(tc, tp),
		kubeclient: kc,
	}
	client, err := r._connect(tc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestCreate_BadRequest(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := &fake.FakeAKSClient{
		MockCreate: func(s string, spec AKSClusterSpec) (cluster containerservice.ManagedCluster, e error) {
			return containerservice.ManagedCluster{}, autorest.DetailedError{StatusCode: http.StatusBadRequest}
		},
	}
	r := Reconciler{
		Client: NewFakeClient(tc),
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(reconcile.Result{}))
}

func TestCreate_ServerError(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := &fake.FakeAKSClient{
		MockCreate: func(s string, spec AKSClusterSpec) (cluster containerservice.ManagedCluster, e error) {
			return containerservice.ManagedCluster{}, autorest.DetailedError{StatusCode: http.StatusInternalServerError}
		},
	}
	r := Reconciler{
		Client: NewFakeClient(tc),
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(reconcile.Result{Requeue: true}))
}

func TestCreate_Success(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	ac := &fake.FakeAKSClient{
		MockCreate: func(s string, spec AKSClusterSpec) (cluster containerservice.ManagedCluster, e error) {
			return containerservice.ManagedCluster{Name: util.String("foo-bar")}, nil
		},
	}
	r := Reconciler{
		Client: NewFakeClient(tc),
	}
	rs, err := r._create(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rs).To(Equal(reconcile.Result{}))

	// assert reconciled cluster
	rc := &AKSCluster{}
	g.Expect(r.Get(ctx, key, rc)).To(Succeed())
	g.Expect(rc.Status.ClusterName).To(Equal("foo-bar"))
	g.Expect(rc.Status.IsCreating()).To(BeTrue())
	g.Expect(rc.Status.State).To(Equal(ClusterStateProvisioning))
}

func aksClusterWithState(state string) containerservice.ManagedCluster {
	return containerservice.ManagedCluster{
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			ProvisioningState: util.String(state),
		},
	}
}

func TestSync_FailedToGetAKS(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to retrieve aks cluster"
	ac := &fake.FakeAKSClient{
		MockGet: func(s string, s2 string) (cluster containerservice.ManagedCluster, e error) {
			return containerservice.ManagedCluster{}, fmt.Errorf(testError)
		},
	}
	r := &Reconciler{
		Client: NewFakeClient(tc),
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncCluster, testError)

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_ClusterNotReady(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	ac := &fake.FakeAKSClient{
		MockGet: func(string, string) (containerservice.ManagedCluster, error) {
			return aksClusterWithState(ClusterStateProvisioning), nil
		},
	}
	r := &Reconciler{
		Client: NewFakeClient(tc),
	}
	expectedStatus := tc.Status.ConditionedStatus

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_FailedToRetrieveAKSCreds(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to retrieve aks creds"
	ac := &fake.FakeAKSClient{
		MockGet: func(string, string) (containerservice.ManagedCluster, error) {
			return aksClusterWithState(ClusterStateSucceeded), nil
		},
	}
	r := &Reconciler{
		Client: NewFakeClient(tc),
		secret: func(cluster *AKSCluster, client aks.Client) (*corev1.Secret, error) {
			return nil, fmt.Errorf(testError)
		},
	}

	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncCluster, testError)

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus, t.Name())
}

func TestSync_FailedToCreateConnectionSecret(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to create connection secret"
	ac := &fake.FakeAKSClient{
		MockGet: func(string, string) (containerservice.ManagedCluster, error) {
			return aksClusterWithState(ClusterStateSucceeded), nil
		},
	}
	kc := NewSimpleClientset()
	kc.PrependReactor("get", "secrets", func(Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: kc,
		secret: func(cluster *AKSCluster, client aks.Client) (*corev1.Secret, error) {
			return &corev1.Secret{}, nil
		},
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorSyncCluster, testError)

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

	ac := &fake.FakeAKSClient{
		MockGet: func(string, string) (containerservice.ManagedCluster, error) {
			return aksClusterWithState(ClusterStateSucceeded), nil
		},
	}
	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
		secret: func(cluster *AKSCluster, client aks.Client) (*corev1.Secret, error) {
			return testSecret, nil
		},
	}
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetReady()

	rs, err := r._sync(tc, ac)
	g.Expect(rs).To(Equal(result))
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

func TestDelete_ReclaimDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tc := testCluster()
	tc.Finalizers = []string{finalizer}
	tc.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	tc.Status.SetReady()

	r := &Reconciler{
		Client:     NewFakeClient(tc),
		kubeclient: NewSimpleClientset(),
	}

	ac := &fake.FakeAKSClient{}
	ac.MockDelete = func(string, string) error {
		return nil
	}

	// expected to have a cond condition set to inactive
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetDeleting()

	rs, err := r._delete(tc, ac)
	g.Expect(rs).To(Equal(result))
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
	g.Expect(rs).To(Equal(result))
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

	ac := &fake.FakeAKSClient{}

	// expected to have all conditions set to inactive
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetDeleting()

	rs, err := r._delete(tc, ac)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())

	assertResource(g, r, expectedStatus, t.Name())
}

func TestDelete_Failed(t *testing.T) {
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

	ac := &fake.FakeAKSClient{
		MockDelete: func(string, string) error {
			return fmt.Errorf(testError)
		},
	}

	rs, err := r._delete(tc, ac)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	// expected status
	expectedStatus := tc.Status.ConditionedStatus
	expectedStatus.UnsetAllConditions()
	expectedStatus.SetFailed(errorDeleteCluster, testError)

	assertResource(g, r, expectedStatus, t.Name())
}

func TestReconcile_ObjectNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	r := &Reconciler{
		Client: NewFakeClient(),
	}
	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
}

func TestReconcile_ClientError(t *testing.T) {
	g := NewGomegaWithT(t)
	testError := "test-client-error"
	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*AKSCluster) (aks.Client, error) {
			return nil, fmt.Errorf(testError)
		},
	}
	// expected to have a failed condition
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorClusterClient, testError)

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
		connect: func(*AKSCluster) (aks.Client, error) {
			return nil, nil
		},
		delete: func(*AKSCluster, aks.Client) (reconcile.Result, error) {
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
	var rc *AKSCluster
	r := &Reconciler{
		Client:     NewFakeClient(testCluster()),
		kubeclient: NewSimpleClientset(),
		connect: func(*AKSCluster) (aks.Client, error) {
			return nil, nil
		},
		create: func(cluster *AKSCluster, client aks.Client) (reconcile.Result, error) {
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
		connect: func(*AKSCluster) (aks.Client, error) {
			return nil, nil
		},
		sync: func(*AKSCluster, aks.Client) (reconcile.Result, error) {
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

func TestSecret_FailedListCredentials(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.SetCreating()

	testError := "failed to list cluster kubeconfig creds"
	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{}, fmt.Errorf(testError)
		},
	}
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
	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{
				Value: &data,
			}, nil
		},
	}
	r := &Reconciler{}
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

	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{
				Value: &data,
			}, nil
		},
	}
	r := &Reconciler{}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestSecret_KubeconfigClusterNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(strings.Replace(kubeconfig, "name: cluster-name", "name: foo", -1))

	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{
				Value: &data,
			}, nil
		},
	}
	r := &Reconciler{}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestSecret_KubeconfigAuthInfoNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(strings.Replace(kubeconfig, "name: cluster-user", "name: foo", -1))

	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{
				Value: &data,
			}, nil
		},
	}
	r := &Reconciler{}
	s, err := r._secret(tc, ac)
	g.Expect(s).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}
func TestSecret_Success(t *testing.T) {
	g := NewGomegaWithT(t)
	tc := testCluster()
	tc.Status.ClusterName = "cluster-context"

	data := []byte(kubeconfig)

	ac := &fake.FakeAKSClient{
		MockListCredentials: func(s string, s2 string) (credentialResult containerservice.CredentialResult, e error) {
			return containerservice.CredentialResult{
				Value: &data,
			}, nil
		},
	}
	r := &Reconciler{}
	s, err := r._secret(tc, ac)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s).NotTo(BeNil())

	g.Expect(s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]).To(Equal([]byte("https://cluster-name:443")))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretCAKey]))).To(Equal("certificate-authority-data"))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey]))).To(Equal("client-certificate-data"))
	g.Expect(strings.TrimSpace(string(s.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey]))).To(Equal("client-key-data"))
}

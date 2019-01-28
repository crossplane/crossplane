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
limitations under the License
*/

package workload

import (
	"testing"
	"time"

	"github.com/crossplaneio/crossplane/pkg/apis/compute"
	. "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	. "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace = "default"
	name      = "test"
)

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
	objectMeta = metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
)

func init() {
	_ = compute.AddToScheme(scheme.Scheme)
}

func testSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: objectMeta,
	}
}

func testCluster() *KubernetesCluster {
	return &KubernetesCluster{
		ObjectMeta: objectMeta,
		Spec:       KubernetesClusterSpec{},
	}
}

func testWorkload() *Workload {
	return &Workload{
		ObjectMeta: objectMeta,
		Spec:       WorkloadSpec{},
	}
}

func TestWorkloadCreatePredicate(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(CreatePredicate(event.CreateEvent{})).To(BeFalse())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &Workload{}})).To(BeFalse())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &Workload{
		Status: WorkloadStatus{Cluster: &corev1.ObjectReference{}}}})).To(BeTrue())
}

func TestReconcileNotScheduled(t *testing.T) {
	g := NewGomegaWithT(t)

	w := testWorkload()
	expCondition := corev1alpha1.ConditionedStatus{}
	r := &Reconciler{
		Client: fake.NewFakeClient(w),
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(r.Get(nil, key, w)).ShouldNot(HaveOccurred())
	g.Expect(w.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expCondition))
}

func TestReconcileClientError(t *testing.T) {
	g := NewGomegaWithT(t)

	w := testWorkload()
	w.Status.Cluster = &corev1.ObjectReference{}
	testError := "test-client-error"
	expCondition := corev1alpha1.ConditionedStatus{}
	expCondition.SetFailed(errorClusterClient, testError)
	r := &Reconciler{
		Client: fake.NewFakeClient(w),
		connect: func(*Workload) (i kubernetes.Interface, e error) {
			return nil, errors.New(testError)
		},
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))
	g.Expect(r.Get(nil, key, w)).ShouldNot(HaveOccurred())
	g.Expect(w.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expCondition))
}

func TestReconcileDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	dt := metav1.NewTime(time.Now())
	w := testWorkload()
	w.DeletionTimestamp = &dt
	w.Status.Cluster = &corev1.ObjectReference{}
	r := &Reconciler{
		Client:  fake.NewFakeClient(w),
		connect: func(*Workload) (i kubernetes.Interface, e error) { return nil, nil },
		delete: func(workload *Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
			return resultDone, nil
		},
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
}

func TestReconcileCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	w := testWorkload()
	w.Status.Cluster = &corev1.ObjectReference{}
	r := &Reconciler{
		Client:  fake.NewFakeClient(w),
		connect: func(*Workload) (i kubernetes.Interface, e error) { return nil, nil },
		create: func(workload *Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
			return resultDone, nil
		},
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
}

func TestReconcileSync(t *testing.T) {
	g := NewGomegaWithT(t)

	w := testWorkload()
	w.Status.Cluster = &corev1.ObjectReference{}
	w.Status.State = WorkloadStateRunning
	r := &Reconciler{
		Client:  fake.NewFakeClient(w),
		connect: func(*Workload) (i kubernetes.Interface, e error) { return nil, nil },
		sync: func(workload *Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
			return resultDone, nil
		},
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
}

func TestConnectNotScheduled(t *testing.T) {
	g := NewGomegaWithT(t)

	w := testWorkload()
	r := &Reconciler{
		Client: fake.NewFakeClient(w),
	}

	_, err := r._connect(w)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("workload is not scheduled"))
}

func TestConnectNoCluster(t *testing.T) {
	g := NewGomegaWithT(t)

	c := testCluster()
	w := testWorkload()
	w.Status.Cluster = c.ObjectReference()
	r := &Reconciler{
		Client: fake.NewFakeClient(w),
	}

	_, err := r._connect(w)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("kubernetesclusters.compute.crossplane.io \"test\" not found"))
}

func TestConnectNoClusterSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	c := testCluster()
	w := testWorkload()
	r := &Reconciler{
		Client:     fake.NewFakeClient(c, w),
		kubeclient: NewSimpleClientset(),
	}
	w.Status.Cluster = c.ObjectReference()

	_, err := r._connect(w)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("secrets \"test\" not found"))
}

func TestConnectNoClusterSecretHostValue(t *testing.T) {
	g := NewGomegaWithT(t)

	s := testSecret()
	s.Data = make(map[string][]byte)
	c := testCluster()
	w := testWorkload()
	r := &Reconciler{
		Client:     fake.NewFakeClient(w),
		kubeclient: NewSimpleClientset(s),
	}
	g.Expect(r.Client.Create(nil, c)).ShouldNot(HaveOccurred())
	g.Expect(r.Client.Get(nil, key, c)).ShouldNot(HaveOccurred())
	w.Status.Cluster = c.ObjectReference()

	_, err := r._connect(w)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("kubernetes cluster endpoint/host is not found"))
}

func TestConnectInvalidSecretHostValue(t *testing.T) {
	g := NewGomegaWithT(t)

	s := testSecret()
	s.Data = map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("foo :bar"),
	}
	c := testCluster()
	w := testWorkload()
	r := &Reconciler{
		Client:     fake.NewFakeClient(w),
		kubeclient: NewSimpleClientset(s),
	}
	g.Expect(r.Client.Create(nil, c)).ShouldNot(HaveOccurred())
	g.Expect(r.Client.Get(nil, key, c)).ShouldNot(HaveOccurred())
	w.Status.Cluster = c.ObjectReference()

	_, err := r._connect(w)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("cannot parse Kubernetes endpoint as URL: parse foo :bar: first path segment in URL cannot contain colon"))
}

func TestConnect(t *testing.T) {
	g := NewGomegaWithT(t)

	s := testSecret()
	s.Data = map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("https://foo.bar"),
	}
	c := testCluster()
	w := testWorkload()
	r := &Reconciler{
		Client:     fake.NewFakeClient(w),
		kubeclient: NewSimpleClientset(s),
	}
	g.Expect(r.Client.Create(nil, c)).ShouldNot(HaveOccurred())
	g.Expect(r.Client.Get(nil, key, c)).ShouldNot(HaveOccurred())
	w.Status.Cluster = c.ObjectReference()

	k, err := r._connect(w)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(k).ShouldNot(BeNil())
}

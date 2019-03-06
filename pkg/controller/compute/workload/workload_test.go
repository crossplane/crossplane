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

package workload

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	. "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	. "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/compute"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
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

func testCluster() *computev1alpha1.KubernetesCluster {
	return &computev1alpha1.KubernetesCluster{
		ObjectMeta: objectMeta,
		Spec:       computev1alpha1.KubernetesClusterSpec{},
	}
}

func testWorkload() *computev1alpha1.Workload {
	return &computev1alpha1.Workload{
		ObjectMeta: objectMeta,
		Spec:       computev1alpha1.WorkloadSpec{},
	}
}

func testDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-deployment",
			UID:       "test-deployment-uid",
		},
	}
}

func testService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-service",
			UID:       "test-service-uid",
		},
	}
}

func TestWorkloadCreatePredicate(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(CreatePredicate(event.CreateEvent{})).To(BeFalse())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &computev1alpha1.Workload{}})).To(BeFalse())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &computev1alpha1.Workload{
		Status: computev1alpha1.WorkloadStatus{Cluster: &corev1.ObjectReference{}}}})).To(BeTrue())
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
		connect: func(*computev1alpha1.Workload) (i kubernetes.Interface, e error) {
			return nil, fmt.Errorf(testError)
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
		connect: func(*computev1alpha1.Workload) (i kubernetes.Interface, e error) { return nil, nil },
		delete: func(workload *computev1alpha1.Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
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
		connect: func(*computev1alpha1.Workload) (i kubernetes.Interface, e error) { return nil, nil },
		create: func(workload *computev1alpha1.Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
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
	w.Status.State = computev1alpha1.WorkloadStateRunning
	r := &Reconciler{
		Client:  fake.NewFakeClient(w),
		connect: func(*computev1alpha1.Workload) (i kubernetes.Interface, e error) { return nil, nil },
		sync: func(workload *computev1alpha1.Workload, i kubernetes.Interface) (result reconcile.Result, e error) {
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

func Test_addWorkloadReferenceLabel(t *testing.T) {
	g := NewGomegaWithT(t)

	// test workload with test testUid value
	testUID := "test-testUid"

	type args struct {
		m   *metav1.ObjectMeta
		uid string
	}
	tests := []struct {
		name string
		args args
	}{
		{"Nil labels", args{&metav1.ObjectMeta{}, testUID}},
		{"Empty labels", args{&metav1.ObjectMeta{Labels: make(map[string]string)}, testUID}},
		{"Label added", args{&metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}, testUID}},
		{"Label updated", args{&metav1.ObjectMeta{Labels: map[string]string{workloadReferenceLabelKey: "foo-bar"}}, testUID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addWorkloadReferenceLabel(tt.args.m, tt.args.uid)
		})
		g.Expect(tt.args.m.Labels).ShouldNot(BeNil())
		g.Expect(tt.args.m.Labels).Should(HaveKeyWithValue(workloadReferenceLabelKey, string(tt.args.uid)))
	}
}

func Test_getWorkloadReferenceLabel(t *testing.T) {
	g := NewGomegaWithT(t)

	type args struct {
		m metav1.ObjectMeta
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Nil labels", args{metav1.ObjectMeta{}}, ""},
		{"Empty labels", args{metav1.ObjectMeta{Labels: make(map[string]string)}}, ""},
		{"Label not found", args{metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}}, ""},
		{"Label found", args{metav1.ObjectMeta{Labels: map[string]string{workloadReferenceLabelKey: "test-uid"}}}, "test-uid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.Expect(getWorkloadReferenceLabel(tt.args.m)).Should(Equal(tt.want))
		})
	}
}

func Test_propagateDeployment(t *testing.T) {
	g := NewGomegaWithT(t)

	testName := "test-name"
	targetNamespace := "test-ns"
	workloadUID := "test-uid"

	td := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "bar"}, // to test selector update
				},
			},
		},
	}

	// propagate create deployment without namespace value
	client := NewSimpleClientset()
	rd, err := propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rd).ShouldNot(BeNil())
	g.Expect(rd.Labels).Should(HaveKeyWithValue(workloadReferenceLabelKey, workloadUID))
	g.Expect(rd.Spec.Selector).ShouldNot(BeNil())
	g.Expect(rd.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("foo", "bar"))

	// propagate create deployment with name collision
	_, err = propagateDeployment(client, td, targetNamespace, workloadUID+"-2")
	g.Expect(err).Should(MatchError(fmt.Errorf("cannot propagate, deployment %s/%s already exists", td.Namespace, td.Name)))

	// propagate update deployment: add a new label to target deployment to test the update
	td.Labels["foo"] = "bar"
	rd, err = propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rd.Labels).Should(HaveKeyWithValue("foo", "bar"))

	// propagate create deployment with the namespace value different from the workload's target namespace
	td.Namespace = "default"
	client = NewSimpleClientset()
	rd, err = propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	_, err = client.AppsV1().Deployments(targetNamespace).Get(td.Name, metav1.GetOptions{})
	g.Expect(err).Should(HaveOccurred())

	g.Expect(errors.IsNotFound(err)).Should(BeTrue())
	g.Expect(rd.Labels).Should(HaveKeyWithValue(workloadReferenceLabelKey, workloadUID))

	// test client errors
	// GET deployment error
	client = NewSimpleClientset()
	client.PrependReactor("get", "deployments", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client get error")
	})
	_, err = propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client get error"))

	// CREATE deployment error
	client = NewSimpleClientset()
	client.PrependReactor("create", "deployments", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client create error")
	})
	_, err = propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client create error"))

	// UPDATE deployment error
	client = NewSimpleClientset(td)
	client.PrependReactor("update", "deployments", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client update error")
	})
	_, err = propagateDeployment(client, td, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client update error"))
}

func Test_propagateService(t *testing.T) {
	g := NewGomegaWithT(t)

	testName := "test-name"
	targetNamespace := "test-ns"
	workloadUID := "test-uid"

	ts := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}

	// propagate create service without namespace value
	client := NewSimpleClientset()
	rs, err := propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs.Labels).Should(HaveKeyWithValue(workloadReferenceLabelKey, workloadUID))

	// propagate create service with name collision
	_, err = propagateService(client, ts, targetNamespace, workloadUID+"-2")
	g.Expect(err).Should(MatchError(fmt.Errorf("cannot propagate, service %s/%s already exists", ts.Namespace, ts.Name)))

	// propagate update service: add a new label to target service to test the update
	ts.Labels["foo"] = "bar"
	rs, err = propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs.Labels).Should(HaveKeyWithValue("foo", "bar"))

	// propagate create service with the namespace value different from the workload's target namespace
	ts.Namespace = "default"
	client = NewSimpleClientset()
	rs, err = propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs.Namespace).ShouldNot(Equal(targetNamespace))
	g.Expect(rs.Labels).Should(HaveKeyWithValue(workloadReferenceLabelKey, workloadUID))

	// test client errors
	// GET service error
	client = NewSimpleClientset()
	client.PrependReactor("get", "services", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client get error")
	})
	_, err = propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client get error"))

	// CREATE service error
	client = NewSimpleClientset()
	client.PrependReactor("create", "services", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client create error")
	})
	_, err = propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client create error"))

	// UPDATE service error
	client = NewSimpleClientset(ts)
	client.PrependReactor("update", "services", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test client update error")
	})
	_, err = propagateService(client, ts, targetNamespace, workloadUID)
	g.Expect(err).Should(MatchError("test client update error"))

}

func Test_create(t *testing.T) {
	g := NewGomegaWithT(t)
	tw := testWorkload()
	td := testDeployment()
	ts := testService()

	client := NewSimpleClientset()

	r := &Reconciler{
		Client: fake.NewFakeClient(tw),
		propagateDeployment: func(i kubernetes.Interface, deployment *appsv1.Deployment, s string, s2 string) (*appsv1.Deployment, error) {
			return td, nil
		},
		propagateService: func(i kubernetes.Interface, service *corev1.Service, s string, s2 string) (*corev1.Service, error) {
			return ts, nil
		},
	}
	expStatus := tw.Status.ConditionedStatus
	expStatus.SetCreating()

	rs, err := r._create(tw, client)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(tw.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expStatus))
	g.Expect(tw.Status.Deployment.UID).Should(Equal(td.UID))
	g.Expect(tw.Status.Service.UID).Should(Equal(ts.UID))
}

func Test_create_Failures(t *testing.T) {
	g := NewGomegaWithT(t)
	tw := testWorkload()
	client := NewSimpleClientset()

	expStatus := tw.Status.ConditionedStatus
	expStatus.SetCreating()

	// Target namespace error
	testError := "test error creating target namespace"
	client.PrependReactor("create", "namespaces", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})
	expStatus.SetFailed(errorCreating, testError)
	r := &Reconciler{
		Client: fake.NewFakeClient(tw),
	}
	rs, err := r._create(tw, client)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))
	g.Expect(tw.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expStatus))
	client.ReactionChain = client.ReactionChain[:0]

	// Deployment propagation failure
	testError = "test deployment propagation error"
	r = &Reconciler{
		Client: fake.NewFakeClient(tw),
		propagateDeployment: func(i kubernetes.Interface, deployment *appsv1.Deployment, s string, s2 string) (*appsv1.Deployment, error) {
			return nil, fmt.Errorf(testError)
		},
	}

	expStatus.SetFailed(errorCreating, testError)

	rs, err = r._create(tw, client)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))
	g.Expect(tw.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expStatus))

	// Service propagation failure
	testError = "test service propagation error"
	r.propagateDeployment = func(i kubernetes.Interface, deployment *appsv1.Deployment, s string, s2 string) (*appsv1.Deployment, error) {
		return testDeployment(), nil
	}
	r.propagateService = func(i kubernetes.Interface, deployment *corev1.Service, s string, s2 string) (*corev1.Service, error) {
		return nil, fmt.Errorf(testError)
	}
	expStatus.SetFailed(errorCreating, testError)

	rs, err = r._create(tw, client)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))
	g.Expect(tw.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(expStatus))
}

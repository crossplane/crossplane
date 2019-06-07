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

package scheduler

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/compute"
	. "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	namespace    = "default"
	workloadName = "test-workload"
	clusterName  = "test-cluster"
)

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      workloadName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
)

func init() {
	_ = compute.AddToScheme(scheme.Scheme)
}

func testCluster(ns, name string) *KubernetesCluster {
	return &KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: KubernetesClusterSpec{},
	}
}

func testWorkload() *Workload {
	return &Workload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: namespace,
		},
		Spec: WorkloadSpec{},
	}
}

// TestReconcile
func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	wl := testWorkload()

	r := &Reconciler{
		Client: NewFakeClient(wl),
	}

	// successful scheduling
	r.schedule = func(workload *Workload) (result reconcile.Result, e error) {
		return resultDone, nil
	}
	rs, err := r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))

	// requeueing scheduling
	r.schedule = func(workload *Workload) (result reconcile.Result, e error) {
		return resultRequeue, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))

	// schedule error
	r.schedule = func(workload *Workload) (result reconcile.Result, e error) {
		return resultDone, fmt.Errorf("test-error")
	}
	_, err = r.Reconcile(request)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError("test-error"))

	// already assigned cluster - no scheduling
	wl.Status.Cluster = &corev1.ObjectReference{}
	r.Client = NewFakeClient(wl)
	rs, err = r.Reconcile(request)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))

}

// TestScheduleNoCluster - test workload scheduling against environment with no (0) Kubernetes clusters
func TestScheduleNoClusters(t *testing.T) {
	g := NewGomegaWithT(t)

	wl := testWorkload()
	expStatus := wl.Status.DeprecatedConditionedStatus
	expStatus.SetFailed(errorUnschedulable, "Cannot match to any existing cluster")
	r := &Reconciler{
		Client: NewFakeClient(wl),
	}

	rs, err := r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultRequeue))
	g.Expect(wl.Status.DeprecatedConditionedStatus).Should(corev1alpha1.MatchDeprecatedConditionedStatus(expStatus))
}

// TestScheduleSingleClusterNoSelector - test workload scheduling against environment with a
// single Kubernetes cluster and workload has no cluster selector(s)
func TestScheduleSingleClusterNoSelector(t *testing.T) {
	g := NewGomegaWithT(t)

	wl := testWorkload()
	cl := testCluster(namespace, clusterName)
	expStatus := wl.Status.DeprecatedConditionedStatus
	r := &Reconciler{
		Client: NewFakeClient(wl, cl),
	}

	rs, err := r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(wl.Status.DeprecatedConditionedStatus).Should(corev1alpha1.MatchDeprecatedConditionedStatus(expStatus))
	g.Expect(wl.Status.Cluster).Should(Equal(meta.ReferenceTo(cl)))
}

// TestScheduleRoundRobin - schedule workload against multiple matching cluster
func TestScheduleRoundRobin(t *testing.T) {
	g := NewGomegaWithT(t)

	wl := testWorkload()
	clA := testCluster(namespace, clusterName)
	clB := testCluster("foo", "bar")

	r := &Reconciler{
		Client: NewFakeClient(wl, clA, clB),
	}

	rs, err := r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(wl.Status.Cluster).Should(Equal(meta.ReferenceTo(clA)))

	// repeat scheduling and assert workload is scheduled on a different cluster
	wl.Status.Cluster = nil
	rs, err = r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(wl.Status.Cluster).Should(Equal(meta.ReferenceTo(clB)))

}

// TestScheduleSelector - schedule workload against multiple (2) clusters, where only single cluster
// has matching labels
// Expected: workload should be consistently scheduled on the same, matched cluster
func TestScheduleSelector(t *testing.T) {
	// We are skipping this test due to pending fix in controller-runtime
	// https://github.com/kubernetes-sigs/controller-runtime/issues/293
	t.Skip()

	g := NewGomegaWithT(t)

	wl := testWorkload()
	wl.Spec.ClusterSelector = map[string]string{
		"foo": "bar",
	}

	clA := testCluster(namespace, clusterName)
	clA.Labels = map[string]string{
		"foo": "bar",
	}
	clB := testCluster("foo", "bar")

	r := &Reconciler{
		Client: NewFakeClient(wl, clA, clB),
	}

	rs, err := r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(wl.Status.Cluster).Should(Equal(meta.ReferenceTo(clA)))

	wl.Status.Cluster = nil
	rs, err = r._schedule(wl)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rs).Should(Equal(resultDone))
	g.Expect(wl.Status.Cluster).Should(Equal(meta.ReferenceTo(clA)))

}

func TestWorkloadCreatePredicate(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(CreatePredicate(event.CreateEvent{})).To(BeFalse())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &Workload{}})).To(BeTrue())
	g.Expect(CreatePredicate(event.CreateEvent{Object: &Workload{
		Status: WorkloadStatus{Cluster: &corev1.ObjectReference{}}}})).To(BeFalse())
}

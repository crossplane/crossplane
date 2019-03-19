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
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/log"
)

const (
	controllerName = "scheduler.compute.crossplane.io"

	errorUnschedulable = "Unschedulable"
)

var (
	logger        = log.Log.WithName("controller." + controllerName)
	ctx           = context.Background()
	resultDone    = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Add creates a new Instance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	client.Client
	scheme           *runtime.Scheme
	recorder         record.EventRecorder
	lastClusterIndex uint64

	schedule func(*computev1alpha1.Workload) (reconcile.Result, error)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetRecorder(controllerName),
	}
	r.schedule = r._schedule
	return r
}

// CreatePredicate accepts Workload instances with set `Status.Cluster` reference value
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*computev1alpha1.Workload)
	if !ok {
		return false
	}
	return wl.Status.Cluster == nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Instance
	return c.Watch(&source.Kind{Type: &computev1alpha1.Workload{}}, &handler.EnqueueRequestForObject{}, &predicate.Funcs{CreateFunc: CreatePredicate})
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *computev1alpha1.Workload, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return resultRequeue, r.Status().Update(ctx, instance)
}

// _schedule assigns Workload to a matching cluster. If the workload matches more than one cluster use
// round-robin to select next cluster
func (r *Reconciler) _schedule(instance *computev1alpha1.Workload) (reconcile.Result, error) {

	clusters := &computev1alpha1.KubernetesClusterList{}

	if err := r.List(context.Background(), client.MatchingLabels(instance.Spec.ClusterSelector), clusters); err != nil {
		return resultDone, err
	}

	if len(clusters.Items) == 0 {
		return r.fail(instance, errorUnschedulable, "Cannot match to any existing cluster")
	}

	// round-robin cluster index selection
	index := int(r.lastClusterIndex % uint64(len(clusters.Items)))
	cluster := clusters.Items[index]
	r.lastClusterIndex++

	// save target cluster into status
	instance.Status.Cluster = cluster.ObjectReference()

	return resultDone, r.Status().Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger.V(1).Info("reconciling", "kind", computev1alpha1.WorkloadKindAPIVersion, "request", request)
	// fetch the CRD instance
	instance := &computev1alpha1.Workload{}

	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return resultDone, nil
		}
		return resultDone, err
	}

	if instance.Status.Cluster == nil {
		return r.schedule(instance)
	}

	return resultDone, nil
}

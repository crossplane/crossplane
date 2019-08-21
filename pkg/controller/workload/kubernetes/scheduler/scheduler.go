/*
Copyright 2019 The Crossplane Authors.

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
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	workloadv1alpha1 "github.com/crossplaneio/crossplane/apis/workload/v1alpha1"
)

const (
	controllerName   = "scheduler.workload.crossplane.io"
	reconcileTimeout = 1 * time.Minute
	requeueOnSuccess = 2 * time.Minute
)

var log = logging.Logger.WithName("controller." + controllerName)

type scheduler interface {
	schedule(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) reconcile.Result
}

type roundRobinScheduler struct {
	kube             client.Client
	lastClusterIndex uint64
}

func (s *roundRobinScheduler) schedule(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) reconcile.Result {
	app.Status.State = workloadv1alpha1.KubernetesApplicationStatePending

	clusters := &computev1alpha1.KubernetesClusterList{}
	if err := s.kube.List(ctx, clusters, client.MatchingLabels(app.Spec.ClusterSelector.MatchLabels)); err != nil {
		app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{Requeue: true}
	}

	if len(clusters.Items) == 0 {
		app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}
	}

	// Round-robin cluster selection
	index := int(s.lastClusterIndex % uint64(len(clusters.Items)))
	cluster := clusters.Items[index]
	s.lastClusterIndex++

	app.Status.Cluster = meta.ReferenceTo(&cluster, computev1alpha1.KubernetesClusterGroupVersionKind)
	app.Status.State = workloadv1alpha1.KubernetesApplicationStateScheduled
	app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

	return reconcile.Result{Requeue: false}
}

// CreatePredicate accepts KubernetesApplications that have not yet been
// scheduled to a KubernetesCluster.
func CreatePredicate(e event.CreateEvent) bool {
	wl, ok := e.Object.(*workloadv1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Status.Cluster == nil
}

// UpdatePredicate accepts KubernetesApplications that have not yet been
// scheduled to a KubernetesCluster.
func UpdatePredicate(e event.UpdateEvent) bool {
	wl, ok := e.ObjectNew.(*workloadv1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Status.Cluster == nil
}

// Controller is responsible for adding the Scheduler
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		kube:      mgr.GetClient(),
		scheduler: &roundRobinScheduler{kube: mgr.GetClient()},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&workloadv1alpha1.KubernetesApplication{}).
		WithEventFilter(&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate}).
		Complete(r)
}

// A Reconciler schedules KubernetesApplications to KubernetesClusters.
type Reconciler struct {
	kube      client.Client
	scheduler scheduler
}

// Reconcile attempts to schedule a KubernetesApplication to a KubernetesCluster
// that matches its cluster selector.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", workloadv1alpha1.KubernetesApplicationKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	app := &workloadv1alpha1.KubernetesApplication{}
	if err := r.kube.Get(ctx, req.NamespacedName, app); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", workloadv1alpha1.KubernetesApplicationKind, req.NamespacedName)
	}

	// This application has been deleted.
	if app.GetDeletionTimestamp() != nil {
		return reconcile.Result{Requeue: false}, nil
	}

	// Someone already scheduled this application.
	if app.Status.Cluster != nil {
		return reconcile.Result{RequeueAfter: requeueOnSuccess}, nil
	}

	return r.scheduler.schedule(ctx, app), errors.Wrapf(r.kube.Update(ctx, app), "cannot update %s %s", workloadv1alpha1.KubernetesApplicationKind, req.NamespacedName)
}

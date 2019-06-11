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
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	workloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerName   = "scheduler.workload.crossplane.io"
	reconcileTimeout = 1 * time.Minute

	reasonUnschedulable = "failed to schedule " + workloadv1alpha1.KubernetesApplicationKind
	errorNoclusters     = "no clusters matched label selector"
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
	app.Status.SetPending()

	sel, err := metav1.LabelSelectorAsSelector(app.Spec.ClusterSelector)
	if err != nil {
		app.Status.SetFailed(reasonUnschedulable, err.Error())
		return reconcile.Result{Requeue: true}
	}

	clusters := &computev1alpha1.KubernetesClusterList{}
	if err := s.kube.List(ctx, &client.ListOptions{LabelSelector: sel}, clusters); err != nil {
		app.Status.SetFailed(reasonUnschedulable, err.Error())
		return reconcile.Result{Requeue: true}
	}

	if len(clusters.Items) == 0 {
		// TODO(negz): Do we really want to set the status to failed here? We
		// 'failed' to schedule only because no clusters match yet. Remaining in
		// pending may be more appropriate.
		app.Status.SetFailed(reasonUnschedulable, errorNoclusters)
		return reconcile.Result{Requeue: true}
	}

	// Round-robin cluster selection
	index := int(s.lastClusterIndex % uint64(len(clusters.Items)))
	cluster := clusters.Items[index]
	s.lastClusterIndex++

	app.Status.Cluster = meta.ReferenceTo(&cluster, computev1alpha1.KubernetesClusterGroupVersionKind)
	app.Status.State = workloadv1alpha1.KubernetesApplicationStateScheduled
	app.Status.UnsetAllDeprecatedConditions()
	app.Status.SetReady()

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

// Add the KubernetesApplication scheduler reconciler to the supplied manager.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		kube:      mgr.GetClient(),
		scheduler: &roundRobinScheduler{kube: mgr.GetClient()},
	}

	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &workloadv1alpha1.KubernetesApplication{}},
		&handler.EnqueueRequestForObject{},
		&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate},
	)
	return errors.Wrapf(err, "cannot watch for %s", v1alpha1.KubernetesApplicationKind)
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
		return reconcile.Result{RequeueAfter: core.RequeueOnSuccess}, nil
	}

	return r.scheduler.schedule(ctx, app), errors.Wrapf(r.kube.Update(ctx, app), "cannot update %s %s", workloadv1alpha1.KubernetesApplicationKind, req.NamespacedName)
}

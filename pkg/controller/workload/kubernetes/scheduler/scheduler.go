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
	"strings"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	reconcileTimeout = 1 * time.Minute
	shortWait        = 30 * time.Second

	errUpdateKubernetesApplicationStatus = "failed to update KubernetesApplication status"
	errUpdateKubernetesApplication       = "failed to update KubernetesApplication"
	errNoUsableTargets                   = "failed to find a usable KubernetesTarget for scheduling"
	errListTargets                       = "failed to list KubernetesTargets in KubernetesApplication namespace"
)

type scheduler interface {
	schedule(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) error
}

type roundRobinScheduler struct {
	kube            client.Client
	lastTargetIndex uint64
}

func (s *roundRobinScheduler) schedule(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) error {
	var targetLabels map[string]string
	if app.Spec.TargetSelector != nil {
		targetLabels = app.Spec.TargetSelector.MatchLabels
	}
	clusters := &workloadv1alpha1.KubernetesTargetList{}
	if err := s.kube.List(ctx, clusters, client.InNamespace(app.GetNamespace()), client.MatchingLabels(targetLabels)); err != nil {
		return errors.Wrap(err, errListTargets)
	}

	// Filter out KubernetesTargets that don't specify a connection
	// secret. We can't run a workload on a cluster that we can't connect to.
	usable := make([]workloadv1alpha1.KubernetesTarget, 0)
	for _, c := range clusters.Items {
		if c.GetWriteConnectionSecretToReference() != nil {
			usable = append(usable, c)
		}
	}

	if len(usable) == 0 {
		return errors.New(errNoUsableTargets)
	}

	// Round-robin target selection
	index := int(s.lastTargetIndex % uint64(len(usable)))
	target := usable[index]
	s.lastTargetIndex++

	app.Spec.Target = &workloadv1alpha1.KubernetesTargetReference{Name: target.Name}
	return errors.Wrap(s.kube.Update(ctx, app), errUpdateKubernetesApplication)
}

// CreatePredicate accepts KubernetesApplications that have not yet been
// scheduled to a KubernetesTarget.
func CreatePredicate(e event.CreateEvent) bool {
	wl, ok := e.Object.(*workloadv1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Spec.Target == nil
}

// UpdatePredicate accepts KubernetesApplications that have not yet been
// scheduled to a KubernetesTarget.
func UpdatePredicate(e event.UpdateEvent) bool {
	wl, ok := e.ObjectNew.(*workloadv1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Spec.Target == nil
}

// Setup adds a controller that schedules KubernetesApplications.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := "scheduler/" + strings.ToLower(workloadv1alpha1.KubernetesApplicationGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&workloadv1alpha1.KubernetesApplication{}).
		WithEventFilter(&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate}).
		Complete(&Reconciler{
			kube:      mgr.GetClient(),
			scheduler: &roundRobinScheduler{kube: mgr.GetClient()},
			log:       l.WithValues("controller", name),
		})
}

// A Reconciler schedules KubernetesApplications to KubernetesTargets.
type Reconciler struct {
	kube      client.Client
	scheduler scheduler
	log       logging.Logger
}

// Reconcile attempts to schedule a KubernetesApplication to a KubernetesTarget
// that matches its cluster selector.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	r.log.Debug("Reconciling", "request", req)

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
	if app.Spec.Target != nil {
		return reconcile.Result{}, nil
	}

	if err := r.scheduler.schedule(ctx, app); err != nil {
		app.Status.State = workloadv1alpha1.KubernetesApplicationStatePending
		app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.kube.Status().Update(ctx, app), errUpdateKubernetesApplicationStatus)
	}

	app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	app.Status.State = workloadv1alpha1.KubernetesApplicationStateScheduled
	return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, app), errUpdateKubernetesApplicationStatus)
}

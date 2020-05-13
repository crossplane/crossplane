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
limitations under the License.
*/

package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	reconcileTimeout = 1 * time.Minute
	shortWait        = 30 * time.Second
	longWait         = 1 * time.Minute

	errGarbageCollect = "failed to garbage collect KubernetesApplicationResources"
	errSyncTemplate   = "failed to sync template with KubernetesApplicationResource"
)

type syncer interface {
	sync(ctx context.Context, app *v1alpha1.KubernetesApplication) (v1alpha1.KubernetesApplicationState, error)
}

// CreatePredicate accepts KubernetesApplications that have been scheduled to a
// KubernetesTarget.
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*v1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Spec.Target != nil
}

// UpdatePredicate accepts KubernetesApplications that have been scheduled to a
// KubernetesTarget.
func UpdatePredicate(event event.UpdateEvent) bool {
	wl, ok := event.ObjectNew.(*v1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Spec.Target != nil
}

// Setup adds a controller that reconciles KubernetesApplications.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := "workload/" + strings.ToLower(v1alpha1.KubernetesApplicationGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.KubernetesApplication{}).
		Owns(&v1alpha1.KubernetesApplicationResource{}).
		WithEventFilter(&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate}).
		Complete(&Reconciler{
			kube: mgr.GetClient(),
			local: &localCluster{
				ar: &applicationResourceClient{kube: mgr.GetClient()},
				gc: &applicationResourceGarbageCollector{kube: mgr.GetClient()},
			},
			log: l.WithValues("controller", name),
		})
}

// localCluster is a syncDeleter that syncs and deletes resources from the same
// cluster as their controlling application.
type localCluster struct {
	ar applicationResourceSyncer
	gc garbageCollector
}

func (c *localCluster) sync(ctx context.Context, app *v1alpha1.KubernetesApplication) (v1alpha1.KubernetesApplicationState, error) {
	var errs []error

	// If App was deleted, do not attempt to sync. The KubernetesApplication is
	// blocked on deletion of all KubernetesApplicationResources that have
	// controller references to it. If we attempt to create or update while a
	// subset of those KubernetesApplicationResources have not yet been deleted,
	// we may be creating a resource that we intend to be deleted.
	if meta.WasDeleted(app) {
		return v1alpha1.KubernetesApplicationStateDeleted, nil
	}

	// Garbage collect any resource we control but no longer have templates for.
	if err := c.gc.process(ctx, app); err != nil {
		return v1alpha1.KubernetesApplicationStateFailed, errors.Wrap(err, errGarbageCollect)
	}

	app.Status.DesiredResources = len(app.Spec.ResourceTemplates)
	app.Status.SubmittedResources = 0

	// Create or update all resources with extant templates.
	for i := range app.Spec.ResourceTemplates {
		submitted, err := c.ar.sync(ctx, renderTemplate(app, &app.Spec.ResourceTemplates[i]))

		if submitted {
			app.Status.SubmittedResources++
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == app.Status.DesiredResources {
		return v1alpha1.KubernetesApplicationStateFailed, errors.Wrap(condenseErrors(errs), errSyncTemplate)
	}

	if app.Status.SubmittedResources == 0 {
		return v1alpha1.KubernetesApplicationStateScheduled, nil
	}

	if app.Status.SubmittedResources < app.Status.DesiredResources {
		return v1alpha1.KubernetesApplicationStatePartial, errors.Wrap(condenseErrors(errs), errSyncTemplate)
	}

	return v1alpha1.KubernetesApplicationStateSubmitted, nil
}

// renderTemplate produces a KubernetesApplicationResource from the supplied
// KubernetesApplicationResourceTemplate. Note that we somewhat confusingly
// also refer to the output KubernetesApplicationResource as a 'template' when
// it is passed to an applicationResourceSyncer.
func renderTemplate(app *v1alpha1.KubernetesApplication, template *v1alpha1.KubernetesApplicationResourceTemplate) *v1alpha1.KubernetesApplicationResource {
	ref := metav1.NewControllerRef(app, v1alpha1.KubernetesApplicationGroupVersionKind)

	ar := &v1alpha1.KubernetesApplicationResource{}
	ar.SetName(template.GetName())
	ar.SetNamespace(app.GetNamespace())
	ar.SetOwnerReferences([]metav1.OwnerReference{*ref})
	ar.SetLabels(template.GetLabels())
	ar.SetAnnotations(template.GetAnnotations())

	ar.Spec = template.Spec
	ar.Spec.Target = app.Spec.Target
	ar.Status.State = v1alpha1.KubernetesApplicationResourceStateScheduled

	return ar
}

type applicationResourceSyncer interface {
	// sync the supplied template with the Crossplane API server. Returns true
	// if the templated resource exists and has been submitted to its scheduled
	// API server, as well as any error encountered.
	sync(ctx context.Context, template *v1alpha1.KubernetesApplicationResource) (submitted bool, err error)
}

type applicationResourceClient struct {
	kube client.Client
}

func (c *applicationResourceClient) sync(ctx context.Context, template *v1alpha1.KubernetesApplicationResource) (bool, error) {
	submitted := false

	// ApplyOptions are executed in the order that they are passed, so by the
	// time we reach the anonymous function we will already know that this KAR
	// is controllable by the KA.
	err := resource.NewAPIUpdatingApplicator(c.kube).Apply(ctx, template, resource.MustBeControllableBy(metav1.GetControllerOf(template).UID), resource.UpdateFn(func(current, _ runtime.Object) {
		c := current.(*v1alpha1.KubernetesApplicationResource)
		if c.Status.State == v1alpha1.KubernetesApplicationResourceStateSubmitted {
			submitted = true
		}

		c.SetLabels(template.GetLabels())
		c.SetAnnotations(template.GetAnnotations())
		c.Spec = *template.Spec.DeepCopy()

		// By the time we get here Apply will have already checked that this KAR
		// is controllable by the KA in the MustBeControllableBy ApplyOption, so
		// it is safe to overwrite the owner references with the template's.
		c.SetOwnerReferences(template.GetOwnerReferences())
	}))
	return submitted, errors.Wrapf(err, "cannot sync %s", v1alpha1.KubernetesApplicationResourceKind)
}

type garbageCollector interface {
	// process garbage collection of the supplied app.
	process(ctx context.Context, app *v1alpha1.KubernetesApplication) error
}

type applicationResourceGarbageCollector struct {
	kube client.Client
}

func (gc *applicationResourceGarbageCollector) process(ctx context.Context, app *v1alpha1.KubernetesApplication) error {
	desired := map[string]bool{}
	for _, t := range app.Spec.ResourceTemplates {
		desired[t.GetName()] = true
	}

	// Grab a list of all resources in our namespace.
	resources := &v1alpha1.KubernetesApplicationResourceList{}
	if err := gc.kube.List(ctx, resources, client.InNamespace(app.GetNamespace())); err != nil {
		return errors.Wrapf(err, "cannot garbage collect %s", v1alpha1.KubernetesApplicationResourceKind)
	}

	// Delete any resources we control that do not match an extant template.
	// We presume, because we are the controller of these resources, that we
	// created them but their templates have been removed.
	for i := range resources.Items {
		ar := &resources.Items[i]

		// We don't control this resource.
		if c := metav1.GetControllerOf(ar); c == nil || c.UID != app.GetUID() {
			continue
		}

		// This resource exists in one of our templates.
		if desired[ar.GetName()] {
			continue
		}

		// We control this resource but we don't have a template for it.
		if err := gc.kube.Delete(ctx, ar); err != nil && !kerrors.IsNotFound(err) {
			app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		}
	}

	return nil
}

// A Reconciler reconciles KubernetesApplications.
type Reconciler struct {
	kube  client.Client
	local syncer
	log   logging.Logger
}

// Reconcile scheduled KubernetesApplications by managing their templated
// KubernetesApplicationResources.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	r.log.Debug("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	app := &v1alpha1.KubernetesApplication{}
	if err := r.kube.Get(ctx, req.NamespacedName, app); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", v1alpha1.KubernetesApplicationKind, req.NamespacedName)
	}

	state, err := r.local.sync(ctx, app)
	if err != nil {
		app.Status.State = state
		app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(r.kube.Status().Update(ctx, app), "cannot update status %s %s", v1alpha1.KubernetesApplicationKind, req.NamespacedName)
	}

	app.Status.State = state
	app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrapf(r.kube.Status().Update(ctx, app), "cannot update status %s %s", v1alpha1.KubernetesApplicationKind, req.NamespacedName)
}

func getControllerName(obj metav1.Object) string {
	c := metav1.GetControllerOf(obj)
	if c == nil {
		return ""
	}

	return c.Name
}

func condenseErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", errs)
}

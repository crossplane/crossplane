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
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane/apis/workload/v1alpha1"
)

const (
	controllerName   = "kubernetesapplication.workload.crossplane.io"
	reconcileTimeout = 1 * time.Minute
	requeueOnWait    = 30 * time.Second
)

var log = logging.Logger.WithName("controller." + controllerName)

type syncer interface {
	sync(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result
}

// CreatePredicate accepts KubernetesApplications that have been scheduled to a
// KubernetesCluster.
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*v1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Status.Cluster != nil
}

// UpdatePredicate accepts KubernetesApplications that have been scheduled to a
// KubernetesCluster.
func UpdatePredicate(event event.UpdateEvent) bool {
	wl, ok := event.ObjectNew.(*v1alpha1.KubernetesApplication)
	if !ok {
		return false
	}
	return wl.Status.Cluster != nil
}

// Add the KubernetesApplication scheduler reconciler to the supplied manager.
// Reconcilers are triggered when either the application or any of its resources
// change.
func Add(mgr manager.Manager) error {
	kube := mgr.GetClient()
	r := &Reconciler{
		kube: kube,
		local: &localCluster{
			ar: &applicationResourceClient{kube: kube},
			gc: &applicationResourceGarbageCollector{kube: kube},
		},
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.KubernetesApplicationResource{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.KubernetesApplication{}, IsController: true},
	); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.KubernetesApplicationResourceKind)
	}

	err = c.Watch(
		&source.Kind{Type: &v1alpha1.KubernetesApplication{}},
		&handler.EnqueueRequestForObject{},
		&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate},
	)
	return errors.Wrapf(err, "cannot watch for %s", v1alpha1.KubernetesApplicationKind)
}

// Controller is responsible for adding the KubernetesApplication
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	kube := mgr.GetClient()
	r := &Reconciler{
		kube: kube,
		local: &localCluster{
			ar: &applicationResourceClient{kube: kube},
			gc: &applicationResourceGarbageCollector{kube: kube},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.KubernetesApplication{}).
		Owns(&v1alpha1.KubernetesApplicationResource{}).
		WithEventFilter(&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate}).
		Complete(r)
}

// localCluster is a syncDeleter that syncs and deletes resources from the same
// cluster as their controlling application.
type localCluster struct {
	ar applicationResourceSyncer
	gc garbageCollector
}

func (c *localCluster) sync(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result {
	app.Status.DesiredResources = len(app.Spec.ResourceTemplates)
	app.Status.SubmittedResources = 0

	// Garbage collect any resource we control but no longer have templates for.
	if err := c.gc.process(ctx, app); err != nil {
		app.Status.State = v1alpha1.KubernetesApplicationStateFailed
		app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{Requeue: true}
	}

	// Create or update all resources with extant templates.
	for i := range app.Spec.ResourceTemplates {
		submitted, err := c.ar.sync(ctx, renderTemplate(app, &app.Spec.ResourceTemplates[i]))

		if submitted {
			app.Status.SubmittedResources++
		}

		if err != nil {
			app.Status.State = v1alpha1.KubernetesApplicationStateFailed
			app.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
			return reconcile.Result{Requeue: true}
		}
	}

	if app.Status.SubmittedResources == 0 {
		// Note we set _state_ scheduled, and _status_ pending here. The pending
		// state and status have different meanings; the former means "pending
		// scheduling to a Kubernetes cluster" while the latter means "pending
		// successful reconciliation".
		app.Status.State = v1alpha1.KubernetesApplicationStateScheduled
		app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: requeueOnWait}
	}

	if app.Status.SubmittedResources < app.Status.DesiredResources {
		app.Status.State = v1alpha1.KubernetesApplicationStatePartial
		app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: requeueOnWait}
	}

	app.Status.State = v1alpha1.KubernetesApplicationStateSubmitted
	app.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}
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
	ar.Status.Cluster = app.Status.Cluster
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
	// We make a copy of our template here so we can compare the template as
	// passed to this method with the remote resource.
	remote := template.DeepCopy()

	submitted := false
	_, err := util.CreateOrUpdate(ctx, c.kube, remote, func() error {
		// Inside this anonymous function ar could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.

		if !meta.HaveSameController(remote, template) {
			return errors.Errorf("%s %s exists and is not controlled by %s %s",
				v1alpha1.KubernetesApplicationResourceKind, remote.GetName(),
				v1alpha1.KubernetesApplicationKind, getControllerName(template))
		}

		if remote.Status.State == v1alpha1.KubernetesApplicationResourceStateSubmitted {
			submitted = true
		}

		remote.SetLabels(template.GetLabels())
		remote.SetAnnotations(template.GetAnnotations())
		remote.Spec = *template.Spec.DeepCopy()

		return nil
	})

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
}

// Reconcile scheduled KubernetesApplications by managing their templated
// KubernetesApplicationResources.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.KubernetesApplicationKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	app := &v1alpha1.KubernetesApplication{}
	if err := r.kube.Get(ctx, req.NamespacedName, app); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", v1alpha1.KubernetesApplicationKind, req.NamespacedName)
	}

	return r.local.sync(ctx, app), errors.Wrapf(r.kube.Update(ctx, app), "cannot update %s %s", v1alpha1.KubernetesApplicationKind, req.NamespacedName)
}

func getControllerName(obj metav1.Object) string {
	c := metav1.GetControllerOf(obj)
	if c == nil {
		return ""
	}

	return c.Name
}

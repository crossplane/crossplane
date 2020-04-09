/*
Copyright 2020 The Crossplane Authors.

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

package applicationconfiguration

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

const (
	reconcileTimeout = 1 * time.Minute
	shortWait        = 30 * time.Second
	longWait         = 1 * time.Minute
)

// Reconcile error strings.
const (
	errGetAppConfig          = "cannot get application configuration"
	errUpdateAppConfigStatus = "cannot update application configuration status"
	errRenderComponents      = "cannot render components"
	errApplyComponents       = "cannot apply components"
	errGCComponent           = "cannot garbage collect components"
)

// Reconcile event reasons.
const (
	reasonRenderComponents = "RenderedComponents"
	reasonApplyComponents  = "AppliedComponents"
	reasonGGComponent      = "GarbageCollectedComponent"

	reasonCannotRenderComponents = "CannotRenderComponents"
	reasonCannotApplyComponents  = "CannotApplyComponents"
	reasonCannotGGComponents     = "CannotGarbageCollectComponents"
)

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := "oam/" + strings.ToLower(v1alpha2.ApplicationConfigurationGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha2.ApplicationConfiguration{}).
		Complete(NewReconciler(mgr,
			WithLogger(l.WithValues("controller", name)),
			WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

// A Reconciler reconciles OAM ApplicationConfigurations by rendering and
// instantiating their Components and Traits.
type Reconciler struct {
	client     client.Client
	components ComponentRenderer
	workloads  WorkloadApplicator
	gc         GarbageCollector

	log    logging.Logger
	record event.Recorder
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithRenderer specifies how the Reconciler should render workloads and traits.
func WithRenderer(r ComponentRenderer) ReconcilerOption {
	return func(rc *Reconciler) {
		rc.components = r
	}
}

// WithApplicator specifies how the Reconciler should apply workloads and traits.
func WithApplicator(a WorkloadApplicator) ReconcilerOption {
	return func(rc *Reconciler) {
		rc.workloads = a
	}
}

// WithGarbageCollector specifies how the Reconciler should garbage collect
// workloads and traits when an ApplicationConfiguration is edited to remove
// them.
func WithGarbageCollector(gc GarbageCollector) ReconcilerOption {
	return func(rc *Reconciler) {
		rc.gc = gc
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(l logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = l
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// NewReconciler returns a Reconciler that reconciles ApplicationConfigurations
// by rendering and instantiating their Components and Traits.
func NewReconciler(m ctrl.Manager, o ...ReconcilerOption) *Reconciler {
	// NOTE(negz): We take a ctrl.Manager here despite only using its client for
	// consistency with other NewReconciler functions, and to avoid needing to
	// change the signature if we eventually need more from the manager (e.g its
	// scheme).

	r := &Reconciler{
		client: m.GetClient(),
		components: &components{
			client:   m.GetClient(),
			params:   ParameterResolveFn(resolve),
			workload: ResourceRenderFn(renderWorkload),
			trait:    ResourceRenderFn(renderTrait),
		},
		workloads: &workloads{
			client: resource.NewAPIPatchingApplicator(m.GetClient()),
		},
		gc:     GarbageCollectorFn(eligible),
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// NOTE(negz): We don't validate anything against their definitions at the
// controller level. We assume this will be done by validating admission
// webhooks.

// Reconcile an OAM ApplicationConfigurations by rendering and instantiating its
// Components and Traits.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	ac := &v1alpha2.ApplicationConfiguration{}
	if err := r.client.Get(ctx, req.NamespacedName, ac); err != nil {
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetAppConfig)
	}

	log = log.WithValues("uid", ac.GetUID(), "version", ac.GetResourceVersion())

	workloads, err := r.components.Render(ctx, ac)
	if err != nil {
		log.Debug("Cannot render components", "error", err, "requeue-after", time.Now().Add(shortWait))
		r.record.Event(ac, event.Warning(reasonCannotRenderComponents, err))
		ac.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errRenderComponents)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, ac), errUpdateAppConfigStatus)
	}
	log.Debug("Successfully rendered components", "workloads", len(workloads))
	r.record.Event(ac, event.Normal(reasonRenderComponents, "Successfully rendered components", "workloads", strconv.Itoa(len(workloads))))

	if err := r.workloads.Apply(ctx, workloads, resource.MustBeControllableBy(ac.GetUID())); err != nil {
		log.Debug("Cannot apply components", "error", err, "requeue-after", time.Now().Add(shortWait))
		r.record.Event(ac, event.Warning(reasonCannotApplyComponents, err))
		ac.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errApplyComponents)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, ac), errUpdateAppConfigStatus)
	}
	log.Debug("Successfully applied components", "workloads", len(workloads))
	r.record.Event(ac, event.Normal(reasonApplyComponents, "Successfully applied components", "workloads", strconv.Itoa(len(workloads))))

	// Kubernetes garbage collection will (by default) reap workloads and traits
	// when the appconfig that controls them (in the controller reference sense)
	// is deleted. Here we cover the case in which a component or one of its
	// traits is removed from an extant appconfig.
	for _, e := range r.gc.Eligible(ac.GetNamespace(), ac.Status.Workloads, workloads) {
		// https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		e := e

		log := log.WithValues("kind", e.GetKind(), "name", e.GetName())
		record := r.record.WithAnnotations("kind", e.GetKind(), "name", e.GetName())

		if err := r.client.Delete(ctx, &e); resource.IgnoreNotFound(err) != nil {
			log.Debug("Cannot garbage collect component", "error", err, "requeue-after", time.Now().Add(shortWait))
			record.Event(ac, event.Warning(reasonCannotGGComponents, err))
			ac.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errGCComponent)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, ac), errUpdateAppConfigStatus)
		}
		log.Debug("Garbage collected resource")
		record.Event(ac, event.Normal(reasonGGComponent, "Successfully garbage collected component"))
	}

	ac.Status.Workloads = make([]v1alpha2.WorkloadStatus, len(workloads))
	for i := range workloads {
		ac.Status.Workloads[i] = workloads[i].Status()
	}

	ac.SetConditions(v1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, ac), errUpdateAppConfigStatus)
}

// A Workload produced by an OAM ApplicationConfiguration.
type Workload struct {
	// ComponentName that produced this workload.
	ComponentName string

	// A Workload object.
	Workload *unstructured.Unstructured

	// Traits associated with this workload.
	Traits []unstructured.Unstructured
}

// Status produces the status of this workload and its traits, suitable for use
// in the status of an ApplicationConfiguration.
func (w Workload) Status() v1alpha2.WorkloadStatus {
	acw := v1alpha2.WorkloadStatus{
		ComponentName: w.ComponentName,
		Reference: runtimev1alpha1.TypedReference{
			APIVersion: w.Workload.GetAPIVersion(),
			Kind:       w.Workload.GetKind(),
			Name:       w.Workload.GetName(),
		},
		Traits: make([]v1alpha2.WorkloadTrait, len(w.Traits)),
	}
	for i := range w.Traits {
		acw.Traits[i].Reference = runtimev1alpha1.TypedReference{
			APIVersion: w.Traits[i].GetAPIVersion(),
			Kind:       w.Traits[i].GetKind(),
			Name:       w.Traits[i].GetName(),
		}
	}
	return acw
}

// A GarbageCollector returns resource eligible for garbage collection. A
// resource is considered eligible if a reference exists in the supplied slice
// of workload statuses, but not in the supplied slice of workloads.
type GarbageCollector interface {
	Eligible(namespace string, ws []v1alpha2.WorkloadStatus, w []Workload) []unstructured.Unstructured
}

// A GarbageCollectorFn returns resource eligible for garbage collection.
type GarbageCollectorFn func(namespace string, ws []v1alpha2.WorkloadStatus, w []Workload) []unstructured.Unstructured

// Eligible resources.
func (fn GarbageCollectorFn) Eligible(namespace string, ws []v1alpha2.WorkloadStatus, w []Workload) []unstructured.Unstructured {
	return fn(namespace, ws, w)
}

func eligible(namespace string, ws []v1alpha2.WorkloadStatus, w []Workload) []unstructured.Unstructured {
	applied := make(map[runtimev1alpha1.TypedReference]bool)
	for _, wl := range w {
		r := runtimev1alpha1.TypedReference{
			APIVersion: wl.Workload.GetAPIVersion(),
			Kind:       wl.Workload.GetKind(),
			Name:       wl.Workload.GetName(),
		}
		applied[r] = true
		for _, t := range wl.Traits {
			r := runtimev1alpha1.TypedReference{
				APIVersion: t.GetAPIVersion(),
				Kind:       t.GetKind(),
				Name:       t.GetName(),
			}
			applied[r] = true
		}
	}
	eligible := make([]unstructured.Unstructured, 0)
	for _, s := range ws {

		if !applied[s.Reference] {
			w := &unstructured.Unstructured{}
			w.SetAPIVersion(s.Reference.APIVersion)
			w.SetKind(s.Reference.Kind)
			w.SetNamespace(namespace)
			w.SetName(s.Reference.Name)
			eligible = append(eligible, *w)
		}

		for _, ts := range s.Traits {
			if !applied[ts.Reference] {
				t := &unstructured.Unstructured{}
				t.SetAPIVersion(ts.Reference.APIVersion)
				t.SetKind(ts.Reference.Kind)
				t.SetNamespace(namespace)
				t.SetName(ts.Reference.Name)
				eligible = append(eligible, *t)
			}
		}
	}

	return eligible
}

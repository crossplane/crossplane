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

// Package offered manages the lifecycle of claim controllers.
package offered

import (
	"context"
	"fmt"
	"strings"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	secretsv1alpha1 "github.com/crossplane/crossplane/apis/secrets/v1alpha1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/claim"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	timeout   = 1 * time.Minute
	finalizer = "offered.apiextensions.crossplane.io"
)

// Error strings.
const (
	errGetXRD          = "cannot get CompositeResourceDefinition"
	errRenderCRD       = "cannot render composite resource claim CustomResourceDefinition"
	errGetCRD          = "cannot get composite resource claim CustomResourceDefinition"
	errApplyCRD        = "cannot apply rendered composite resource claim CustomResourceDefinition"
	errUpdateStatus    = "cannot update status of CompositeResourceDefinition"
	errStartController = "cannot start composite resource claim controller"
	errAddFinalizer    = "cannot add composite resource claim finalizer"
	errRemoveFinalizer = "cannot remove composite resource claim finalizer"
	errDeleteCRD       = "cannot delete composite resource claim CustomResourceDefinition"
	errListCRs         = "cannot list defined composite resource claims"
	errDeleteCR        = "cannot delete defined composite resource claim"
)

// Wait strings.
const (
	waitCRDelete     = "waiting for defined composite resource claims to be deleted"
	waitCRDEstablish = "waiting for composite resource claim CustomResourceDefinition to be established"
)

// Event reasons.
const (
	reasonRenderCRD event.Reason = "RenderCRD"
	reasonOfferXRC  event.Reason = "OfferClaim"
	reasonRedactXRC event.Reason = "RedactClaim"
)

// A ControllerEngine can start and stop Kubernetes controllers on demand.
type ControllerEngine interface {
	IsRunning(name string) bool
	Start(name string, o kcontroller.Options, w ...controller.Watch) error
	Stop(name string)
	Err(name string) error
}

// A CRDRenderer renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)

// Render the supplied CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	return fn(d)
}

// Setup adds a controller that reconciles CompositeResourceDefinitions by
// defining a composite resource claim and starting a controller to reconcile
// it.
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
	name := "offered/" + strings.ToLower(v1.CompositeResourceDefinitionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithOptions(o))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.CompositeResourceDefinition{}).
		Owns(&extv1.CustomResourceDefinition{}).
		WithEventFilter(resource.NewPredicates(OffersClaim())).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithOptions lets the Reconciler know which options to pass to new composite
// resource claim controllers.
func WithOptions(o apiextensionscontroller.Options) ReconcilerOption {
	return func(r *Reconciler) {
		r.options = o
	}
}

// WithFinalizer specifies how the Reconciler should finalize
// CompositeResourceDefinitions.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Finalizer = f
	}
}

// WithControllerEngine specifies how the Reconciler should manage the
// lifecycles of claim controllers.
func WithControllerEngine(c ControllerEngine) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.ControllerEngine = c
	}
}

// WithCRDRenderer specifies how the Reconciler should render a
// CompositeResourceDefinition's corresponding CustomResourceDefinition.
func WithCRDRenderer(c CRDRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.CRDRenderer = c
	}
}

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// NewReconciler returns a Reconciler of CompositeResourceDefinitions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		mgr: mgr,

		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIUpdatingApplicator(kube),
		},

		claim: definition{
			CRDRenderer:      CRDRenderFn(xcrd.ForCompositeResourceClaim),
			ControllerEngine: controller.NewEngine(mgr),
			Finalizer:        resource.NewAPIFinalizer(kube, finalizer),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),

		options: apiextensionscontroller.Options{
			Options: controller.DefaultOptions(),
		},
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

type definition struct {
	CRDRenderer
	ControllerEngine
	resource.Finalizer
}

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	mgr    manager.Manager
	client resource.ClientApplicator

	claim definition

	log    logging.Logger
	record event.Recorder

	options apiextensionscontroller.Options
}

// Reconcile a CompositeResourceDefinition by defining a new kind of composite
// resource claim and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are complex. Be wary of adding more.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := &v1.CompositeResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		log.Debug(errGetXRD, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetXRD)
	}

	log = log.WithValues(
		"uid", d.GetUID(),
		"version", d.GetResourceVersion(),
		"name", d.GetName(),
	)

	crd, err := r.claim.Render(d)
	if err != nil {
		err = errors.Wrap(err, errRenderCRD)
		r.record.Event(d, event.Warning(reasonRenderCRD, err))
		return reconcile.Result{}, err
	}

	if meta.WasDeleted(d) {
		d.Status.SetConditions(v1.TerminatingClaim())
		if err := r.client.Status().Update(ctx, d); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errUpdateStatus)
			r.record.Event(d, event.Warning(reasonRedactXRC, err))
			return reconcile.Result{}, err
		}

		nn := types.NamespacedName{Name: crd.GetName()}
		if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
			err = errors.Wrap(err, errGetCRD)
			r.record.Event(d, event.Warning(reasonRedactXRC, err))
			return reconcile.Result{}, err
		}

		// The CRD has no creation timestamp, or we don't control it.
		// Most likely we successfully deleted it on a previous
		// reconcile. It's also possible that we're being asked to
		// delete it before we got around to creating it, or that we
		// lost control of it around the same time we were deleted. In
		// the (presumably exceedingly rare) latter case we'll orphan
		// the CRD.
		if !meta.WasCreated(crd) || !metav1.IsControlledBy(crd, d) {
			// It's likely that we've already stopped this
			// controller on a previous reconcile, but we try again
			// just in case. This is a no-op if the controller was
			// already stopped.
			r.claim.Stop(claim.ControllerName(d.GetName()))
			log.Debug("Stopped composite resource claim controller")

			if err := r.claim.RemoveFinalizer(ctx, d); err != nil {
				if kerrors.IsConflict(err) {
					return reconcile.Result{Requeue: true}, nil
				}
				err = errors.Wrap(err, errRemoveFinalizer)
				r.record.Event(d, event.Warning(reasonRedactXRC, err))
				return reconcile.Result{}, err
			}

			// We're all done deleting and have removed our
			// finalizer. There's no need to requeue because there's
			// nothing left to do.
			return reconcile.Result{Requeue: false}, nil
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(d.GetClaimGroupVersionKind())
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			err = errors.Wrap(err, errListCRs)
			r.record.Event(d, event.Warning(reasonRedactXRC, err))
			return reconcile.Result{}, err
		}

		// Ensure all the custom resources we defined are gone before
		// stopping the controller we started to reconcile them. This
		// ensures the controller has a chance to execute its cleanup
		// logic, if any.
		if len(l.Items) > 0 {
			// TODO(negz): DeleteAllOf does not work here, despite
			// working in the definition controller. Could this be
			// due to claims being namespaced rather than cluster
			// scoped?
			for i := range l.Items {
				if err := r.client.Delete(ctx, &l.Items[i]); resource.IgnoreNotFound(err) != nil {
					err = errors.Wrap(err, errDeleteCR)
					r.record.Event(d, event.Warning(reasonRedactXRC, err))
					return reconcile.Result{}, err
				}
			}

			// We requeue to confirm that all the custom resources
			// we just deleted are actually gone. We need to requeue
			// because we won't be requeued implicitly when the CRs
			// are deleted.
			r.record.Event(d, event.Normal(reasonRedactXRC, waitCRDelete))
			return reconcile.Result{Requeue: true}, nil
		}

		// The controller should be stopped before the deletion of CRD
		// so that it doesn't crash.
		r.claim.Stop(claim.ControllerName(d.GetName()))
		log.Debug("Stopped composite resource claim controller")

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			err = errors.Wrap(err, errDeleteCRD)
			r.record.Event(d, event.Warning(reasonRedactXRC, err))
			return reconcile.Result{}, err
		}
		r.record.Event(d, event.Normal(reasonRedactXRC, fmt.Sprintf("Deleted composite resource claim CustomResourceDefinition: %s", crd.GetName())))

		// We should be requeued implicitly because we're watching the
		// CustomResourceDefinition that we just deleted, but we requeue
		// after just in case the CRD isn't gone after the first
		// requeue.
		return reconcile.Result{Requeue: true}, nil
	}

	if err := r.claim.AddFinalizer(ctx, d); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(d, event.Warning(reasonOfferXRC, err))
		return reconcile.Result{}, err
	}

	origRV := ""
	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(d.GetUID()), resource.StoreCurrentRV(&origRV)); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errApplyCRD)
		r.record.Event(d, event.Warning(reasonOfferXRC, err))
		return reconcile.Result{}, err
	}
	if crd.GetResourceVersion() != origRV {
		r.record.Event(d, event.Normal(reasonOfferXRC, fmt.Sprintf("Applied composite resource claim CustomResourceDefinition: %s", crd.GetName())))
	}

	if !xcrd.IsEstablished(crd.Status) {
		log.Debug(waitCRDEstablish)
		r.record.Event(d, event.Normal(reasonOfferXRC, waitCRDEstablish))
		return reconcile.Result{Requeue: true}, nil
	}

	o := []claim.ReconcilerOption{
		claim.WithLogger(log.WithValues("controller", claim.ControllerName(d.GetName()))),
		claim.WithRecorder(r.record.WithAnnotations("controller", claim.ControllerName(d.GetName()))),
		claim.WithPollInterval(r.options.PollInterval),
	}

	// We only want to use the server-side XR syncer if the relevant feature
	// flag is enabled. Otherwise, we start claim reconcilers with the default
	// client-side syncer. If we use a server-side syncer we also need to handle
	// upgrading fields that were previously managed using client-side apply.
	if r.options.Features.Enabled(features.EnableAlphaClaimSSA) {
		o = append(o,
			claim.WithCompositeSyncer(claim.NewServerSideCompositeSyncer(r.client, names.NewNameGenerator(r.client))),
			claim.WithManagedFieldsUpgrader(claim.NewPatchingManagedFieldsUpgrader(r.client)),
		)
	}

	// We only want to enable ExternalSecretStore support if the relevant
	// feature flag is enabled. Otherwise, we start the Claim reconcilers with
	// their default Connection Propagator.
	if r.options.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		pc := claim.ConnectionPropagatorChain{
			claim.NewAPIConnectionPropagator(r.client),
			connection.NewDetailsManager(r.client, secretsv1alpha1.StoreConfigGroupVersionKind, connection.WithTLSConfig(r.options.ESSOptions.TLSConfig)),
		}

		o = append(o, claim.WithConnectionPropagator(pc), claim.WithConnectionUnpublisher(
			claim.NewSecretStoreConnectionUnpublisher(connection.NewDetailsManager(r.client,
				secretsv1alpha1.StoreConfigGroupVersionKind, connection.WithTLSConfig(r.options.ESSOptions.TLSConfig)))))
	}

	cr := claim.NewReconciler(r.mgr,
		resource.CompositeClaimKind(d.GetClaimGroupVersionKind()),
		resource.CompositeKind(d.GetCompositeGroupVersionKind()), o...)

	ko := r.options.ForControllerRuntime()
	ko.Reconciler = ratelimiter.NewReconciler(claim.ControllerName(d.GetName()), errors.WithSilentRequeueOnConflict(cr), r.options.GlobalRateLimiter)

	if err := r.claim.Err(claim.ControllerName(d.GetName())); err != nil {
		log.Debug("Composite resource controller encountered an error", "error", err)
	}

	observed := d.Status.Controllers.CompositeResourceClaimTypeRef
	desired := v1.TypeReferenceTo(d.GetClaimGroupVersionKind())
	if observed.APIVersion != "" && observed != desired {
		r.claim.Stop(claim.ControllerName(d.GetName()))
		log.Debug("Referenceable version changed; stopped composite resource claim controller",
			"observed-version", observed.APIVersion,
			"desired-version", desired.APIVersion)
	}

	cm := &kunstructured.Unstructured{}
	cm.SetGroupVersionKind(d.GetClaimGroupVersionKind())

	cp := &kunstructured.Unstructured{}
	cp.SetGroupVersionKind(d.GetCompositeGroupVersionKind())

	if err := r.claim.Start(claim.ControllerName(d.GetName()), ko,
		controller.For(cm, &handler.EnqueueRequestForObject{}),
		controller.For(cp, &EnqueueRequestForClaim{}),
	); err != nil {
		err = errors.Wrap(err, errStartController)
		r.record.Event(d, event.Warning(reasonOfferXRC, err))
		return reconcile.Result{}, err
	}
	log.Debug("(Re)started composite resource claim controller")

	d.Status.Controllers.CompositeResourceClaimTypeRef = v1.TypeReferenceTo(d.GetClaimGroupVersionKind())
	d.Status.SetConditions(v1.WatchingClaim())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
}

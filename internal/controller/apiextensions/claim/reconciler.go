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

package claim

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

const (
	finalizer        = "finalizer.apiextensions.crossplane.io"
	reconcileTimeout = 1 * time.Minute
)

// Reasons a composite resource claim is or is not ready.
const (
	ReasonWaiting = "Composite resource claim is waiting for composite resource to become Ready"
)

// Error strings.
const (
	errGetClaim           = "cannot get composite resource claim"
	errGetComposite       = "cannot get referenced composite resource"
	errDeleteComposite    = "cannot delete referenced composite resource"
	errDeleteCDs          = "cannot delete connection details"
	errRemoveFinalizer    = "cannot remove composite resource claim finalizer"
	errAddFinalizer       = "cannot add composite resource claim finalizer"
	errConfigureComposite = "cannot configure composite resource"
	errBindComposite      = "cannot bind composite resource"
	errApplyComposite     = "cannot apply composite resource"
	errConfigureClaim     = "cannot configure composite resource claim"
	errPropagateCDs       = "cannot propagate connection details from composite"

	errUpdateClaimStatus = "cannot update composite resource claim status"
)

// Event reasons.
const (
	reasonBind               event.Reason = "BindCompositeResource"
	reasonDelete             event.Reason = "DeleteCompositeResource"
	reasonCompositeConfigure event.Reason = "ConfigureCompositeResource"
	reasonClaimConfigure     event.Reason = "ConfigureClaim"
	reasonPropagate          event.Reason = "PropagateConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource claim.
func ControllerName(name string) string {
	return "claim/" + name
}

// A Configurator configures the supplied resource, typically either populating the
// composite with fields from the claim, or claim with fields from composite.
type Configurator interface {
	Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// A ConfiguratorFn configures the supplied resource, typically either populating the
// composite with fields from the claim, or claim with fields from composite.
type ConfiguratorFn func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error

// Configure the supplied resource using the supplied claim.
func (fn ConfiguratorFn) Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return fn(ctx, cm, cp)
}

// A Binder binds a composite resource claim to a composite resource.
type Binder interface {
	// Bind the supplied Claim to the supplied Composite resource.
	Bind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// A BinderFn binds a composite resource claim to a composite resource.
type BinderFn func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error

// Bind the supplied Claim to the supplied Composite resource.
func (fn BinderFn) Bind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return fn(ctx, cm, cp)
}

// A ConnectionPropagator is responsible for propagating information required to
// connect to a resource.
type ConnectionPropagator interface {
	PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)
}

// A ConnectionPropagatorFn is responsible for propagating information required
// to connect to a resource.
type ConnectionPropagatorFn func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)

// PropagateConnection details from one resource to the other.
func (fn ConnectionPropagatorFn) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	return fn(ctx, to, from)
}

// A ConnectionPropagatorChain runs multiple connection propagators.
type ConnectionPropagatorChain []ConnectionPropagator

// PropagateConnection details from one resource to the other.
// This method calls PropagateConnection for all ConnectionPropagator's in the
// chain and returns propagated if at least one ConnectionPropagator propagates
// the connection details but exits with an error if any of them fails without
// calling the remaining ones.
func (pc ConnectionPropagatorChain) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	for _, p := range pc {
		var pg bool
		pg, err = p.PropagateConnection(ctx, to, from)
		if pg {
			propagated = true
		}
		if err != nil {
			return propagated, err
		}
	}
	return propagated, nil
}

// A ConnectionUnpublisher is responsible for cleaning up connection secret.
type ConnectionUnpublisher interface {
	// UnpublishConnection details for the supplied Managed resource.
	UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error
}

// A ConnectionUnpublisherFn is responsible for cleaning up connection secret.
type ConnectionUnpublisherFn func(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error

// UnpublishConnection details of a local connection secret owner.
func (fn ConnectionUnpublisherFn) UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error {
	return fn(ctx, so, c)
}

// A Reconciler reconciles composite resource claims by creating exactly one kind of
// concrete composite resource. Each composite resource claim kind should create an instance
// of this controller for each composite resource kind they can bind to, using
// watch predicates to ensure each controller is responsible for exactly one
// type of resource class provisioner. Each controller must watch its subset of
// composite resource claims and any composite resources they control.
type Reconciler struct {
	client       resource.ClientApplicator
	newClaim     func() resource.CompositeClaim
	newComposite func() resource.Composite

	// The below structs embed the set of interfaces used to implement the
	// composite resource claim reconciler. We do this primarily for readability, so that
	// the reconciler logic reads r.composite.Create(), r.claim.Finalize(), etc.
	composite crComposite
	claim     crClaim

	log    logging.Logger
	record event.Recorder
}

type crComposite struct {
	Configurator
	ConnectionPropagator
}

func defaultCRComposite(c client.Client) crComposite {
	return crComposite{
		Configurator:         NewAPIDryRunCompositeConfigurator(c),
		ConnectionPropagator: NewAPIConnectionPropagator(c),
	}
}

type crClaim struct {
	resource.Finalizer
	Binder
	Configurator
	ConnectionUnpublisher
}

func defaultCRClaim(c client.Client) crClaim {
	return crClaim{
		Finalizer:             resource.NewAPIFinalizer(c, finalizer),
		Binder:                NewAPIBinder(c),
		Configurator:          NewAPIClaimConfigurator(c),
		ConnectionUnpublisher: NewNopConnectionUnpublisher(),
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// WithCompositeConfigurator specifies how the Reconciler should configure the bound
// composite resource.
func WithCompositeConfigurator(cf Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configurator = cf
	}
}

// WithClaimConfigurator specifies how the Reconciler should configure the bound
// claim resource.
func WithClaimConfigurator(cf Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Configurator = cf
	}
}

// WithConnectionPropagator specifies which ConnectionPropagator should be used
// to propagate resource connection details to their claim.
func WithConnectionPropagator(p ConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPropagator = p
	}
}

// WithConnectionUnpublisher specifies which ConnectionUnpublisher should be
// used to unpublish resource connection details.
func WithConnectionUnpublisher(u ConnectionUnpublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.ConnectionUnpublisher = u
	}
}

// WithBinder specifies which Binder should be used to bind
// resources to their claim.
func WithBinder(b Binder) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Binder = b
	}
}

// WithClaimFinalizer specifies which ClaimFinalizer should be used to finalize
// claims when they are deleted.
func WithClaimFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Finalizer = f
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

// NewReconciler returns a Reconciler that reconciles composite resource claims of
// the supplied CompositeClaimKind with resources of the supplied CompositeKind.
// The returned Reconciler will apply only the ObjectMetaConfigurator by
// default; most callers should supply one or more CompositeConfigurators to
// configure their composite resources.
func NewReconciler(m manager.Manager, of resource.CompositeClaimKind, with resource.CompositeKind, o ...ReconcilerOption) *Reconciler {
	c := unstructured.NewClient(m.GetClient())
	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		newClaim: func() resource.CompositeClaim {
			return claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind(of)))
		},
		newComposite: func() resource.Composite {
			return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(with)))
		},
		composite: defaultCRComposite(c),
		claim:     defaultCRClaim(c),
		log:       logging.NewNopLogger(),
		record:    event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a composite resource claim with a concrete composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	cm := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, cm); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetClaim, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(cm))
	log = log.WithValues(
		"uid", cm.GetUID(),
		"version", cm.GetResourceVersion(),
		"external-name", meta.GetExternalName(cm),
	)

	cp := r.newComposite()
	if ref := cm.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("composite-name", cm.GetResourceReference().Name)
		log = log.WithValues("composite-name", cm.GetResourceReference().Name)

		if err := r.client.Get(ctx, meta.NamespacedNameOf(ref), cp); resource.IgnoreNotFound(err) != nil {
			log.Debug(errGetComposite, "error", err)
			err = errors.Wrap(err, errGetComposite)
			record.Event(cm, event.Warning(reasonBind, err))
			return reconcile.Result{}, err
		}
	}

	if meta.WasDeleted(cm) {
		log = log.WithValues("deletion-timestamp", cm.GetDeletionTimestamp())

		if meta.WasCreated(cp) {
			ref := cp.GetClaimReference()
			want := meta.ReferenceTo(cm, cm.GetObjectKind().GroupVersionKind())
			if !cmp.Equal(want, ref, cmpopts.IgnoreFields(corev1.ObjectReference{}, "UID")) {
				// We don't requeue (or return an error, which
				// would requeue) in this situation because the
				// claim will need human intervention before we
				// can proceed (e.g. fixing the ref), and we'll
				// be queued implicitly when the claim is
				// edited.
				err := errors.New("refusing to delete composite resource that is not bound to this claim")
				log.Debug(errDeleteComposite, "error", err)
				record.Event(cm, event.Warning(reasonDelete, err))
				return reconcile.Result{Requeue: false}, nil
			}

			if err := r.client.Delete(ctx, cp); resource.IgnoreNotFound(err) != nil {
				log.Debug(errDeleteComposite, "error", err)
				err = errors.Wrap(err, errDeleteComposite)
				record.Event(cm, event.Warning(reasonDelete, err))
				return reconcile.Result{}, err
			}

		}

		// Claims do not publish connection details but may propagate XR
		// secrets. Hence, we need to clean up propagated secrets when the
		// claim is deleted.
		if err := r.claim.UnpublishConnection(ctx, cm, nil); err != nil {
			log.Debug(errDeleteCDs, "error", err)
			err = errors.Wrap(err, errDeleteCDs)
			record.Event(cm, event.Warning(reasonDelete, err))
			return reconcile.Result{}, err
		}

		log.Debug("Successfully deleted composite resource")
		record.Event(cm, event.Normal(reasonDelete, "Successfully deleted composite resource"))

		if err := r.claim.RemoveFinalizer(ctx, cm); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)
			err = errors.Wrap(err, errRemoveFinalizer)
			record.Event(cm, event.Warning(reasonDelete, err))
			return reconcile.Result{}, err
		}

		log.Debug("Successfully deleted composite resource claim")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.claim.AddFinalizer(ctx, cm); err != nil {
		log.Debug(errAddFinalizer, "error")
		err = errors.Wrap(err, errAddFinalizer)
		record.Event(cm, event.Warning(reasonBind, err))
		return reconcile.Result{}, err
	}

	if err := r.composite.Configure(ctx, cm, cp); err != nil {
		log.Debug(errConfigureComposite, "error", err)
		err = errors.Wrap(err, errConfigureComposite)
		record.Event(cm, event.Warning(reasonCompositeConfigure, err))
		return reconcile.Result{}, err
	}

	// We'll know our composite resource's name at this point because it was
	// set by the above configure step.
	record = record.WithAnnotations("composite-name", cp.GetName())
	log = log.WithValues("composite-name", cp.GetName())

	// We want to make sure we bind the claim to the composite (i.e. that we
	// set the claim's resourceRef) before we ever create the composite. We
	// use resourceRef to determine whether or not we need to create a new
	// composite resource. If we first created the composite then set the
	// resourceRef we'd risk leaking composite resources, e.g. if we hit an
	// error between when we created the composite resource and when we
	// persisted its resourceRef.
	if err := r.claim.Bind(ctx, cm, cp); err != nil {
		log.Debug(errBindComposite, "error", err)
		err = errors.Wrap(err, errBindComposite)
		record.Event(cm, event.Warning(reasonBind, err))
		return reconcile.Result{}, err
	}

	if err := r.client.Apply(ctx, cp); err != nil {
		log.Debug(errApplyComposite, "error", err)
		err = errors.Wrap(err, errApplyComposite)
		record.Event(cm, event.Warning(reasonCompositeConfigure, err))
		return reconcile.Result{}, err
	}

	log.Debug("Successfully applied composite resource")
	record.Event(cm, event.Normal(reasonCompositeConfigure, "Successfully applied composite resource"))

	if err := r.claim.Configure(ctx, cm, cp); err != nil {
		log.Debug(errConfigureClaim, "error", err)
		err = errors.Wrap(err, errConfigureClaim)
		record.Event(cm, event.Warning(reasonClaimConfigure, err))
		return reconcile.Result{}, err
	}

	if !resource.IsConditionTrue(cp.GetCondition(xpv1.TypeReady)) {
		log.Debug("Composite resource is not yet ready")
		record.Event(cm, event.Normal(reasonBind, "Composite resource is not yet ready"))

		// We should be watching the composite resource and will have a
		// request queued if it changes, so no need to requeue.
		cm.SetConditions(Waiting())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	log.Debug("Successfully bound composite resource")
	record.Event(cm, event.Normal(reasonBind, "Successfully bound composite resource"))

	propagated, err := r.composite.PropagateConnection(ctx, cm, cp)
	if err != nil {
		log.Debug(errPropagateCDs, "error", err)
		err = errors.Wrap(err, errPropagateCDs)
		record.Event(cm, event.Warning(reasonPropagate, err))
		return reconcile.Result{}, err
	}
	if propagated {
		cm.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		log.Debug("Successfully propagated connection details from composite resource")
		record.Event(cm, event.Normal(reasonPropagate, "Successfully propagated connection details from composite resource"))
	}

	// We have a watch on both the claim and its composite, so there's no
	// need to requeue here.
	cm.SetConditions(xpv1.Available())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
}

// Waiting returns a condition that indicates the composite resource claim is
// currently waiting for its composite resource to become ready.
func Waiting() xpv1.Condition {
	return xpv1.Condition{
		Type:               xpv1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWaiting,
	}
}

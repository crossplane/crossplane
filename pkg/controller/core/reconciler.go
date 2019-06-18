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
limitations under the License.
*/

package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

// RequeueAfter durations.
const (
	AShortWait = 30 * time.Second
)

// Reasons a resource claim is or is not ready.
const (
	ReasonBinding = "Resource claim is waiting for managed resource to become bindable"
)

const (
	controllerName   = "resourceclaim.crossplane.io"
	finalizerName    = "finalizer." + controllerName
	reconcileTimeout = 1 * time.Minute
)

var log = logging.Logger.WithName("controller").WithValues("controller", controllerName)

// A Bindable resource may be bound to another resource. Resources are bindable
// when they available for use.
type Bindable interface {
	SetBindingPhase(v1alpha1.BindingPhase)
	GetBindingPhase() v1alpha1.BindingPhase
}

// A ConditionSetter may have conditions set. Conditions are informational, and
// typically indicate the status of both a resource and its reconciliation
// process.
type ConditionSetter interface {
	SetConditions(c ...v1alpha1.Condition)
}

// A ClaimReferencer may reference a resource claim.
type ClaimReferencer interface {
	SetClaimReference(*corev1.ObjectReference)
	GetClaimReference() *corev1.ObjectReference
}

// A ClassReferencer may reference a resource class.
type ClassReferencer interface {
	SetClassReference(*corev1.ObjectReference)
	GetClassReference() *corev1.ObjectReference
}

// A ResourceReferencer may reference a concrete managed resource.
type ResourceReferencer interface {
	SetResourceReference(*corev1.ObjectReference)
	GetResourceReference() *corev1.ObjectReference
}

// A ConnectionSecretWriterTo may write a connection secret.
type ConnectionSecretWriterTo interface {
	SetWriteConnectionSecretTo(corev1.LocalObjectReference)
	GetWriteConnectionSecretTo() corev1.LocalObjectReference
}

// A Claim (or resource claim) is a Kubernetes object representing an abstract
// resource type (e.g. an SQL database) that may be bound to a concrete managed
// resource (e.g. a CloudSQL instance).
type Claim interface {
	runtime.Object
	metav1.Object

	ClassReferencer
	ResourceReferencer
	ConnectionSecretWriterTo

	ConditionSetter
	Bindable
}

// A Resource (or managed resource) is a Kubernetes object representing a
// concrete managed resource (e.g. a CloudSQL instance).
type Resource interface {
	runtime.Object
	metav1.Object

	ClassReferencer
	ClaimReferencer
	ConnectionSecretWriterTo

	Bindable
}

// A ClaimKind contains the type metadata for a kind of resource claim.
type ClaimKind schema.GroupVersionKind

// A ResourceKind contains the type metadata for a kind of managed resource.
type ResourceKind schema.GroupVersionKind

// A ResourceConfigurator configures a resource, typically by converting it to
// a known type and populating its spec.
type ResourceConfigurator interface {
	Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error
}

// A ResourceCreator creates a resource, typically by submitting it to an API
// server. ResourceCreators must not modify the supplied resource class, but are
// responsible for final modifications to the claim and resource, for example
// ensuring resource, class, claim, and owner references are set.
type ResourceCreator interface {
	Create(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error
}

// A ResourceConnectionPropagator is responsible for propagating information
// required to connect to a managed resource (for example the connection secret)
// from the managed resource to its resource claim.
type ResourceConnectionPropagator interface {
	PropagateConnection(ctx context.Context, cm Claim, rs Resource) error
}

// A ResourceBinder binds a resource claim to a managed resource.
type ResourceBinder interface {
	Bind(ctx context.Context, cm Claim, rs Resource) error
}

// A ResourceDeleter deletes a managed resource when its resource claim is
// deleted.
type ResourceDeleter interface {
	Delete(ctx context.Context, cm Claim, rs Resource) error
}

// A ResourceConfiguratorFn is a function that sastisfies the
// ResourceConfigurator interface.
type ResourceConfiguratorFn func(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error

// Configure the supplied resource using the supplied claim and class.
func (fn ResourceConfiguratorFn) Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error {
	return fn(ctx, cm, cs, rs)
}

// A ConfiguratorChain chains multiple configurators.
type ConfiguratorChain []ResourceConfigurator

// Configure calls each ResourceConfigurator serially. It returns the first
// error it encounters, if any.
func (cc ConfiguratorChain) Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error {
	for _, c := range cc {
		if err := c.Configure(ctx, cm, cs, rs); err != nil {
			return err
		}
	}
	return nil
}

// kindish tries to return the name of the Claim interface's underlying type,
// e.g. rediscluster, or mysqlinstance. Fall back to simply "claim".
func kindish(obj runtime.Object) string {
	if reflect.ValueOf(obj).Type().Kind() != reflect.Ptr {
		return "claim"
	}
	return strings.ToLower(reflect.TypeOf(obj).Elem().Name())
}

// ConfigureObjectMeta sets standard object metadata (i.e. the name and
// namespace) for a dynamically provisioned resource, deriving it from the
// resource claim.
func ConfigureObjectMeta(_ context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error {
	rs.SetNamespace(cm.GetNamespace())
	rs.SetName(fmt.Sprintf("%s-%s", kindish(cm), cm.GetUID()))

	return nil
}

// A Reconciler reconciles resource claims by creating exactly one kind of
// concrete managed resource. Each resource class should create an instance of
// this controller for each managed resource kind they can bind to, using watch
// predicates to ensure each controller is responsible for exactly one type of
// resource claim provisioner. Each controller must watch its subset of resource
// claims and any managed resources they control.
type Reconciler struct {
	client      client.Client
	newClaim    func() Claim
	newResource func() Resource
	resource    resource
}

type resource struct {
	ResourceConfigurator
	ResourceCreator
	ResourceConnectionPropagator
	ResourceBinder
	ResourceDeleter
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithResourceConfigurators specifies which configurators should be used to
// configure each managed resource. Configurators will be applied in the order
// they are specified.
func WithResourceConfigurators(c ...ResourceConfigurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource.ResourceConfigurator = ConfiguratorChain(c)
	}
}

// WithResourceCreator specifies which ResourceCreator should be used to create
// managed resources.
func WithResourceCreator(c ResourceCreator) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource.ResourceCreator = c
	}
}

// WithResourceConnectionPropagator specifies which ResourceConnectionPropagator
// should be used to propagate resource connection details to their claim.
func WithResourceConnectionPropagator(p ResourceConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource.ResourceConnectionPropagator = p
	}
}

// WithResourceBinder specifies which ResourceBinder should be used to bind
// resources to their claim.
func WithResourceBinder(b ResourceBinder) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource.ResourceBinder = b
	}
}

// WithResourceDeleter specifies which ResourceDeleter should be used to delete
// resources when their claim is deleted.
func WithResourceDeleter(d ResourceDeleter) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource.ResourceDeleter = d
	}
}

// NewReconciler returns a Reconciler that reconciles resource claims of the
// supplied ClaimType with resources of the supplied ResourceType. NewReconciler
// panics if asked to reconcile a claim or resource kind that is not registered
// with the supplied manager's scheme. It returns a reconciler that will set
// apply only the ObjectMetaConfigurator by default - most callers should use
// WithResourceConfigurators to configure their resources.
func NewReconciler(m manager.Manager, of ClaimKind, with ResourceKind, o ...ReconcilerOption) *Reconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }
	nr := func() Resource { return MustCreateObject(schema.GroupVersionKind(with), m.GetScheme()).(Resource) }

	// Panic early if we've been asked to reconcile a claim or resource kind
	// that has not been registered with our controller manager's scheme.
	_, _ = nc(), nr()

	r := &Reconciler{
		client:      m.GetClient(),
		newClaim:    nc,
		newResource: nr,
		resource: resource{
			ResourceConfigurator:         ResourceConfiguratorFn(ConfigureObjectMeta),
			ResourceCreator:              &APIResourceCreator{client: m.GetClient(), scheme: m.GetScheme()},
			ResourceConnectionPropagator: &APIResourceConnectionPropagator{client: m.GetClient(), scheme: m.GetScheme()},
			ResourceBinder:               &APIResourceBinder{client: m.GetClient()},
			ResourceDeleter:              &APIResourceDeleter{client: m.GetClient()},
		},
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource claim with a concrete managed resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is a little over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log.V(logging.Debug).Info("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), "cannot get resource claim")
	}

	resource := r.newResource()
	if ref := claim.GetResourceReference(); ref != nil {
		if err := IgnoreNotFound(r.client.Get(ctx, meta.NamespacedNameOf(ref), resource)); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}
	}

	if meta.WasDeleted(claim) {
		if err := r.resource.Delete(ctx, claim, resource); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}

		// We've successfully processed the delete, so there's no further
		// reconciliation to do. There's a good chance our claim no longer
		// exists, but we try update its status just in case it sticks around,
		// for example due to additional finalizers.
		claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), "cannot update resource claim status")
	}

	if !meta.WasCreated(resource) {
		class := &v1alpha1.ResourceClass{}
		if err := r.client.Get(ctx, meta.NamespacedNameOf(claim.GetClassReference()), class); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or the
			// class is (re)created.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}

		if err := r.resource.Configure(ctx, claim, class, resource); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or some
			// issue with the resource class was resolved.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}

		if err := r.resource.Create(ctx, claim, class, resource); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}
	}

	if resource.GetBindingPhase() == v1alpha1.BindingPhaseUnbindable {
		// If this claim was not already binding we'll be requeued due to the
		// status update. Otherwise there's no need to requeue. We should be
		// watching both the resource claims and the resources we own, so we'll
		// be queued if anything changes.
		claim.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
	}

	if resource.GetBindingPhase() == v1alpha1.BindingPhaseUnbound {
		if err := r.resource.PropagateConnection(ctx, claim, resource); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait in case this was a transient error, or the resource connection
			// secret is created.
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}

		if err := r.resource.Bind(ctx, claim, resource); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait, in case this was a transient error.
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: AShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
		}
	}

	// No need to requeue. We should be watching both the resource claims and
	// the resources we own, so we'll be queued if anything changes.
	claim.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), "cannot update resource claim status")
}

// Binding returns a condition that indicates the resource claim is currently
// waiting for its managed resource to become bindable.
func Binding() v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               v1alpha1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonBinding,
	}
}

// TODO(negz): Move the below utility functions elsewhere.

// ResolveClassClaimValues validates the supplied claim value against the
// supplied resource class value. If both are non-zero they must match.
func ResolveClassClaimValues(classValue, claimValue string) (string, error) {
	if classValue == "" {
		return claimValue, nil
	}
	if claimValue == "" {
		return classValue, nil
	}
	if classValue != claimValue {
		return "", fmt.Errorf("claim value [%s] does not match the one defined in the resource class [%s]", claimValue, classValue)
	}
	return claimValue, nil
}

// MustCreateObject returns a new Object of the supplied kind. It panics if the
// kind is unknown to the supplied ObjectCreator.
func MustCreateObject(kind schema.GroupVersionKind, oc runtime.ObjectCreater) runtime.Object {
	obj, err := oc.New(kind)
	if err != nil {
		panic(err)
	}
	return obj
}

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// resource was not found.
func IgnoreNotFound(err error) error {
	if kerrors.IsNotFound(err) {
		return nil
	}
	return err
}

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

package resource

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	managedControllerName   = "managedresource.crossplane.io"
	managedFinalizerName    = "finalizer." + managedControllerName
	managedReconcileTimeout = 1 * time.Minute

	defaultManagedShortWait = 30 * time.Second
	defaultManagedLongWait  = 1 * time.Minute
)

// Error strings.
const (
	errGetManaged = "cannot get managed resource"
)

// ConnectionDetails created or updated during an operation on an external
// resource, for example usernames, passwords, endpoints, ports, etc.
type ConnectionDetails map[string][]byte

// A ManagedPublisher publishes the supplied ConnectionDetails for the supplied
// Managed resource. ManagedPublishers must handle the case in which the
// supplied ConnectionDetails are empty.
type ManagedPublisher interface {
	Publish(ctx context.Context, mg Managed, c ConnectionDetails) error
}

// A ManagedPublisherFn is a function that satisfies the ManagedPublisher interface.
type ManagedPublisherFn func(ctx context.Context, mg Managed, c ConnectionDetails) error

// Publish the supplied ConnectionDetails for the supplied Managed resource.
func (fn ManagedPublisherFn) Publish(ctx context.Context, mg Managed, c ConnectionDetails) error {
	return fn(ctx, mg, c)
}

// A ManagedEstablisher establishes ownership of the supplied Managed resource.
// This typically involves adding a finalizer to the object metadata.
type ManagedEstablisher interface {
	Establish(ctx context.Context, mg Managed) error
}

// An ExternalConnecter produces a new ExternalClient given the supplied
// Managed resource.
type ExternalConnecter interface {
	Connect(ctx context.Context, mg Managed) (ExternalClient, error)
}

// An ExternalClient manages the lifecycle of an external resource.
// None of the calls here should be blocking.
type ExternalClient interface {
	// Observe the external resource the supplied Managed resource represents,
	// if any. Observe implementations must not modify the external resource,
	// but may update the supplied Managed resource to reflect the state of the
	// external resource.
	Observe(ctx context.Context, mg Managed) (ExternalObservation, error)

	// Create an external resource per the specifications of the supplied
	// Managed resource.
	Create(ctx context.Context, mg Managed) (ExternalCreation, error)

	// Update the external resource represented by the supplied Managed
	// resource, if necessary. Update implementations must handle the case in
	// which no update is necessary.
	Update(ctx context.Context, mg Managed) (ExternalUpdate, error)

	// Delete the external resource upon deletion of its associated Managed
	// resource.
	Delete(ctx context.Context, mg Managed) error
}

// A NopConnecter does nothing.
type NopConnecter struct{}

// Connect returns a NopClient. It never returns an error.
func (c *NopConnecter) Connect(_ context.Context, _ Managed) (ExternalClient, error) {
	return &NopClient{}, nil
}

// A NopClient does nothing.
type NopClient struct{}

// Observe does nothing. It returns an empty ExternalObservation and no error.
func (c *NopClient) Observe(ctx context.Context, mg Managed) (ExternalObservation, error) {
	return ExternalObservation{}, nil
}

// Create does nothing. It returns an empty ExternalCreation and no error.
func (c *NopClient) Create(ctx context.Context, mg Managed) (ExternalCreation, error) {
	return ExternalCreation{}, nil
}

// Update does nothing. It returns an empty ExternalUpdate and no error.
func (c *NopClient) Update(ctx context.Context, mg Managed) (ExternalUpdate, error) {
	return ExternalUpdate{}, nil
}

// Delete does nothing. It never returns an error.
func (c *NopClient) Delete(ctx context.Context, mg Managed) error { return nil }

// An ExternalObservation is the result of an observation of an external
// resource.
type ExternalObservation struct {
	ResourceExists    bool
	ConnectionDetails ConnectionDetails
}

// An ExternalCreation is the result of the creation of an external resource.
type ExternalCreation struct {
	ConnectionDetails ConnectionDetails
}

// An ExternalUpdate is the result of an update to an external resource.
type ExternalUpdate struct {
	ConnectionDetails ConnectionDetails
}

// A ManagedReconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type ManagedReconciler struct {
	client     client.Client
	newManaged func() Managed

	shortWait time.Duration
	longWait  time.Duration

	// The below structs embed the set of interfaces used to implement the
	// managed resource reconciler. We do this primarily for readability, so
	// that the reconciler logic reads r.external.Connect(),
	// r.managed.Delete(), etc.
	external mrExternal
	managed  mrManaged
}

type mrManaged struct {
	ManagedPublisher
	ManagedEstablisher
	ManagedFinalizer
}

func defaultMRManaged(m manager.Manager) mrManaged {
	return mrManaged{
		ManagedPublisher:   NewAPISecretPublisher(m.GetClient(), m.GetScheme()),
		ManagedEstablisher: NewAPIManagedFinalizerAdder(m.GetClient()),
		ManagedFinalizer:   NewAPIManagedFinalizerRemover(m.GetClient()),
	}
}

type mrExternal struct {
	ExternalConnecter
}

func defaultMRExternal() mrExternal {
	return mrExternal{
		ExternalConnecter: &NopConnecter{},
	}
}

// A ManagedReconcilerOption configures a ManagedReconciler.
type ManagedReconcilerOption func(*ManagedReconciler)

// WithShortWait specifies how long the ManagedReconciler should wait before
// queueing a new reconciliation in 'short wait' scenarios. The Reconciler
// requeues after a short wait when it knows it is waiting for an external
// operation to complete, or when it encounters a potentially temporary error.
func WithShortWait(after time.Duration) ManagedReconcilerOption {
	return func(r *ManagedReconciler) {
		r.shortWait = after
	}
}

// WithLongWait specifies how long the ManagedReconciler should wait before
// queueing a new reconciliation in 'long wait' scenarios. The Reconciler
// requeues after a long wait when it is not actively waiting for an external
// operation, but wishes to check whether an existing external resource needs to
// be synced to its Crossplane Managed resource.
func WithLongWait(after time.Duration) ManagedReconcilerOption {
	return func(r *ManagedReconciler) {
		r.longWait = after
	}
}

// WithExternalConnecter specifies how the Reconciler should connect to the API
// used to sync and delete external resources.
func WithExternalConnecter(c ExternalConnecter) ManagedReconcilerOption {
	return func(r *ManagedReconciler) {
		r.external.ExternalConnecter = c
	}
}

// WithManagedPublishers specifies how the Reconciler should publish its
// connection details such as credentials and endpoints.
func WithManagedPublishers(p ...ManagedPublisher) ManagedReconcilerOption {
	return func(r *ManagedReconciler) {
		r.managed.ManagedPublisher = PublisherChain(p)
	}
}

// NewManagedReconciler returns a ManagedReconciler that reconciles managed
// resources of the supplied ManagedKind with resources in an external system
// such as a cloud provider API. It panics if asked to reconcile a managed
// resource kind that is not registered with the supplied manager's
// runtime.Scheme. The returned ManagedReconciler reconciles with a dummy, no-op
// 'external system' by default; callers should supply an ExternalConnector that
// returns an ExternalClient capable of managing resources in a real system.
func NewManagedReconciler(m manager.Manager, of ManagedKind, o ...ManagedReconcilerOption) *ManagedReconciler {
	nm := func() Managed { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Managed) }

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_ = nm()

	r := &ManagedReconciler{
		client:     m.GetClient(),
		newManaged: nm,
		shortWait:  defaultManagedShortWait,
		longWait:   defaultManagedLongWait,
		managed:    defaultMRManaged(m),
		external:   defaultMRExternal(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a managed resource with an external resource.
func (r *ManagedReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is a little over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log.V(logging.Debug).Info("Reconciling", "controller", managedControllerName, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), managedReconcileTimeout)
	defer cancel()

	managed := r.newManaged()
	if err := r.client.Get(ctx, req.NamespacedName, managed); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetManaged)
	}

	external, err := r.external.Connect(ctx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider or its secret are missing
		// or invalid. If this is first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	observation, err := external.Observe(ctx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider credentials are invalid
		// or insufficient for observing the external resource type we're
		// concerned with. If this is the first time we encounter this issue
		// we'll be requeued implicitly when we update our status with the new
		// error condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if meta.WasDeleted(managed) {
		// TODO: Reclaim Policy should be used between Claim and Managed. For Managed and External Resource,
		// we need another field.
		if observation.ResourceExists && managed.GetReclaimPolicy() == v1alpha1.ReclaimDelete {
			managed.SetConditions(v1alpha1.Deleting())
			if err := external.Delete(ctx, managed); err != nil {
				// We'll hit this condition if we can't delete our external
				// resource, for example if our provider credentials don't have
				// access to delete it. If this is the first time we encounter this
				// issue we'll be requeued implicitly when we update our status with
				// the new error condition. If not, we want to try again after a
				// short wait.
				managed.SetConditions(v1alpha1.ReconcileError(err))
				return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
			}
		} else {
			if err := r.managed.Finalize(ctx, managed); err != nil {
				// If this is the first time we encounter this issue we'll be
				// requeued implicitly when we update our status with the new error
				// condition. If not, we want to try again after a short wait.
				managed.SetConditions(v1alpha1.ReconcileError(err))
				return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, managed)), errUpdateManagedStatus)
			}
			// We've successfully finalized the deletion of our external and managed
			// resources.
			managed.SetConditions(v1alpha1.ReconcileSuccess())
		}
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, managed)), errUpdateManagedStatus)
	}

	if err := r.managed.Publish(ctx, managed, observation.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if !observation.ResourceExists {
		if err := r.managed.Establish(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we want to try again after a short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		creation, err := external.Create(ctx, managed)
		if err != nil {
			// We'll hit this condition if we can't create our external
			// resource, for example if our provider credentials don't have
			// access to create it. If this is the first time we encounter this
			// issue we'll be requeued implicitly when we update our status with
			// the new error condition. If not, we want to try again after a
			// short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		if err := r.managed.Publish(ctx, managed, creation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we want to try again after a short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully created our external resource. In many cases the
		// creation process takes a little time to finish. We requeue a short
		// wait in order to observe the external resource to determine whether
		// it's ready for use.
		managed.SetConditions(v1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	update, err := external.Update(ctx, managed)
	if err != nil {
		// We'll hit this condition if we can't update our external resource,
		// for example if our provider credentials don't have access to update
		// it. If this is the first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.Publish(ctx, managed, update.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We've successfully attempted to update our external resource. This will
	// often be a no-op. Per the below issue nothing will notify us if and when
	// the external resource we manage changes, so we requeue a speculative
	// reconcile after a long wait in order to observe it and react accordingly.
	// https://github.com/crossplaneio/crossplane/issues/289
	managed.SetConditions(v1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: r.longWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
}

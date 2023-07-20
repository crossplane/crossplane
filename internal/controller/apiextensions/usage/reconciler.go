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

// Package usage manages the lifecycle of usageResource objects.
package usage

import (
	"context"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/usage/dependency"
	"github.com/crossplane/crossplane/internal/usage"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	timeout       = 2 * time.Minute
	finalizer     = "usage.apiextensions.crossplane.io"
	inUseLabelKey = "crossplane.io/in-use"

	errGetUsage                = "cannot get usage"
	errResolveSelectors        = "cannot resolve selectors"
	errListUsages              = "cannot list usages"
	errGetUsing                = "cannot get using"
	errGetUsed                 = "cannot get used"
	errAddOwnerToUsage         = "cannot update usage resource with owner ref"
	errAddLabelAndOwnersToUsed = "cannot update used resource with added label and owners"
	errRemoveOwnerFromUsed     = "cannot update used resource with owner ref removed"
	errAddFinalizer            = "cannot add finalizer"
	errRemoveFinalizer         = "cannot remove finalizer"
	errUpdateStatus            = "cannot update status of usage"
)

// Event reasons.
const (
	reasonResolveSelectors       event.Reason = "ResolveSelectors"
	reasonListUsages             event.Reason = "ListUsages"
	reasonGetUsed                event.Reason = "GetUsedResource"
	reasonGetUsing               event.Reason = "GetUsingResource"
	reasonOwnerRefToUsage        event.Reason = "AddOwnerRefToUsage"
	reasonOwnerRefToUsed         event.Reason = "AddOwnerRefToUsed"
	reasonRemoveOwnerRefFromUsed event.Reason = "RemoveOwnerRefFromUsed"
	reasonAddFinalizer           event.Reason = "AddFinalizer"
	reasonRemoveFinalizer        event.Reason = "RemoveFinalizer"

	reasonUsageConfigured event.Reason = "UsageConfigured"
	reasonWaitUsing       event.Reason = "WaitingUsingDeleted"
)

// Setup adds a controller that reconciles Usages by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
	name := "usage/" + strings.ToLower(v1alpha1.UsageGroupKind)
	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Usage{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
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

type usageResource struct {
	resource.Finalizer
	selectorResolver
}

// NewReconciler returns a Reconciler of Usages.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		mgr: mgr,

		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIUpdatingApplicator(kube),
		},

		usage: usageResource{
			Finalizer:        resource.NewAPIFinalizer(kube, finalizer),
			selectorResolver: newAPISelectorResolver(kube),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),

		pollInterval: 30 * time.Second,
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles Usages.
type Reconciler struct {
	client resource.ClientApplicator
	mgr    manager.Manager

	usage usageResource

	log    logging.Logger
	record event.Recorder

	pollInterval time.Duration
}

// Reconcile a usageResource by defining a new kind of composite
// resource and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcilers are typically complex.
	log := r.log.WithValues("request", req)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get the usageResource resource for this request.
	u := &v1alpha1.Usage{}
	if err := r.client.Get(ctx, req.NamespacedName, u); err != nil {
		log.Debug(errGetUsage, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetUsage)
	}

	if err := r.usage.resolveSelectors(ctx, u); err != nil {
		log.Debug(errResolveSelectors, "error", err)
		err = errors.Wrap(err, errResolveSelectors)
		r.record.Event(u, event.Warning(reasonResolveSelectors, err))
		return reconcile.Result{}, err
	}

	r.record.Event(u, event.Normal(reasonResolveSelectors, "Selectors resolved, if any."))

	of := u.Spec.Of
	by := u.Spec.By

	// Identify used resource as an unstructured object.
	used := dependency.New(dependency.FromReference(v1.ObjectReference{
		Kind:       of.Kind,
		Name:       of.ResourceRef.Name,
		APIVersion: of.APIVersion,
	}))

	if meta.WasDeleted(u) {
		if by != nil {
			// Identify using resource as an unstructured object.
			using := dependency.New(dependency.FromReference(v1.ObjectReference{
				Kind:       by.Kind,
				Name:       by.ResourceRef.Name,
				APIVersion: by.APIVersion,
			}))
			// Get the using resource
			err := r.client.Get(ctx, client.ObjectKey{Name: by.ResourceRef.Name}, using)
			if resource.IgnoreNotFound(err) != nil {
				log.Debug(errGetUsing, "error", err)
				err = errors.Wrap(resource.IgnoreNotFound(err), errGetUsing)
				r.record.Event(u, event.Warning(reasonGetUsing, err))
				return reconcile.Result{}, err
			}

			if l := u.GetLabels()[xcrd.LabelKeyNamePrefixForComposed]; len(l) > 0 && l == using.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] && err == nil {
				// If the usage and using resource are part of the same composite resource, we need to wait for the using resource to be deleted
				msg := "Waiting for using resource to be deleted."
				log.Debug(msg)
				r.record.Event(u, event.Normal(reasonWaitUsing, msg))
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
		// At this point using resource is either:
		// - not defined
		// - not found (deleted)
		// - not part of the same composite resource
		// So, we can proceed with the deletion of the usage.

		// Get the used resource
		var err error
		if err = r.client.Get(ctx, client.ObjectKey{Name: of.ResourceRef.Name}, used); resource.IgnoreNotFound(err) != nil {
			log.Debug(errGetUsed, "error", err)
			err = errors.Wrap(err, errGetUsed)
			r.record.Event(u, event.Warning(reasonGetUsed, err))
			return reconcile.Result{}, err
		}

		// Remove the owner reference from the used resource if it exists
		if err == nil && used.OwnedBy(u.GetUID()) {
			used.RemoveOwnerRef(u.GetUID())
			usageList := &v1alpha1.UsageList{}
			if err = r.client.List(ctx, usageList, client.MatchingFields{usage.InUseIndexKey: usage.IndexValueForObject(used.GetUnstructured())}); err != nil {
				log.Debug(errListUsages, "error", err)
				err = errors.Wrap(err, errListUsages)
				r.record.Event(u, event.Warning(reasonListUsages, err))
				return reconcile.Result{}, errors.Wrap(err, errListUsages)
			}
			// There are no "other" usageResource's referencing the used resource,
			// so we can remove the in-use label from the used resource
			if len(usageList.Items) < 2 {
				meta.RemoveLabels(used, inUseLabelKey)
			}
			if err = r.client.Update(ctx, used); err != nil {
				log.Debug(errRemoveOwnerFromUsed, "error", err)
				err = errors.Wrap(err, errRemoveOwnerFromUsed)
				r.record.Event(u, event.Warning(reasonRemoveOwnerRefFromUsed, err))
				return reconcile.Result{}, err
			}
		}

		// Remove the finalizer from the usage
		if err = r.usage.RemoveFinalizer(ctx, u); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)
			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(u, event.Warning(reasonRemoveFinalizer, err))
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Add finalizer for Usage resource.
	if err := r.usage.AddFinalizer(ctx, u); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(u, event.Warning(reasonAddFinalizer, err))
		return reconcile.Result{}, nil
	}

	// Get the used resource
	if err := r.client.Get(ctx, client.ObjectKey{Name: of.ResourceRef.Name}, used); err != nil {
		log.Debug(errGetUsed, "error", err)
		err = errors.Wrap(err, errGetUsed)
		r.record.Event(u, event.Warning(reasonGetUsed, err))
		return reconcile.Result{}, err
	}

	// Used resource should have in-use label and be owned by the usageResource resource.
	if used.GetLabels()[inUseLabelKey] != "true" || !used.OwnedBy(u.GetUID()) {
		meta.AddLabels(used, map[string]string{inUseLabelKey: "true"})
		meta.AddOwnerReference(used, meta.AsOwner(
			meta.TypedReferenceTo(u, u.GetObjectKind().GroupVersionKind()),
		))
		if err := r.client.Update(ctx, used); err != nil {
			log.Debug(errAddLabelAndOwnersToUsed, "error", err)
			err = errors.Wrap(err, errAddLabelAndOwnersToUsed)
			r.record.Event(u, event.Warning(reasonOwnerRefToUsed, err))
			return reconcile.Result{}, err
		}
	}

	if by != nil {
		// Identify using resource as an unstructured object.
		using := dependency.New(dependency.FromReference(v1.ObjectReference{
			Kind:       by.Kind,
			Name:       by.ResourceRef.Name,
			APIVersion: by.APIVersion,
		}))

		// Get the using resource
		if err := r.client.Get(ctx, client.ObjectKey{Name: by.ResourceRef.Name}, using); err != nil {
			log.Debug(errGetUsing, "error", err)
			err = errors.Wrap(err, errGetUsing)
			r.record.Event(u, event.Warning(reasonGetUsing, err))
			return reconcile.Result{}, err
		}

		// usageResource should have a finalizer and be owned by the using resource.
		if owners := u.GetOwnerReferences(); len(owners) == 0 || owners[0].UID != using.GetUID() {
			meta.AddOwnerReference(u, meta.AsOwner(
				meta.TypedReferenceTo(using, using.GetObjectKind().GroupVersionKind()),
			))
			if err := r.client.Update(ctx, u); err != nil {
				log.Debug(errAddOwnerToUsage, "error", err)
				err = errors.Wrap(err, errAddOwnerToUsage)
				r.record.Event(u, event.Warning(reasonOwnerRefToUsage, err))
				return reconcile.Result{}, err
			}
		}
	}

	// Note(turkenh): Do we really need a Synced condition? Maybe just to be
	// consistent with the other XP resources.
	u.Status.SetConditions(xpv1.ReconcileSuccess())
	u.Status.SetConditions(xpv1.Available())
	r.record.Event(u, event.Normal(reasonUsageConfigured, "Usage configured successfully."))
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, u), errUpdateStatus)
}

/*
Copyright 2023 The Crossplane Authors.

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
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"

	legacy "github.com/crossplane/crossplane/v2/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/v2/apis/protection/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/protection"
	"github.com/crossplane/crossplane/v2/internal/protection/usage"
	"github.com/crossplane/crossplane/v2/internal/xcrd"
)

const (
	reconcileTimeout = 1 * time.Minute
	waitPollInterval = 30 * time.Second
	finalizer        = "usage.apiextensions.crossplane.io"
	// Note(turkenh): In-use label enables the "DELETE" requests on resources
	// with this label to be intercepted by the webhook and rejected if the
	// resource is in use.
	inUseLabelKey        = "crossplane.io/in-use"
	detailsAnnotationKey = "crossplane.io/usage-details"

	errGetUsage             = "cannot get usage"
	errResolveSelectors     = "cannot resolve selectors"
	errFindUsages           = "cannot find usages"
	errGetUsing             = "cannot get using"
	errGetUsed              = "cannot get used"
	errAddOwnerToUsage      = "cannot update usage resource with owner ref"
	errAddDetailsAnnotation = "cannot update usage resource with details annotation"
	errAddInUseLabel        = "cannot add in use label to the used resource"
	errRemoveInUseLabel     = "cannot remove in use label from the used resource"
	errAddFinalizer         = "cannot add finalizer"
	errRemoveFinalizer      = "cannot remove finalizer"
	errUpdateStatus         = "cannot update status of usage"
	errParseAPIVersion      = "cannot parse APIVersion"
)

// Event reasons.
const (
	reasonResolveSelectors event.Reason = "ResolveSelectors"
	reasonFindUsages       event.Reason = "FindUsages"
	reasonGetUsed          event.Reason = "GetUsedResource"
	reasonGetUsing         event.Reason = "GetUsingResource"
	reasonDetailsToUsage   event.Reason = "AddDetailsToUsage"
	reasonOwnerRefToUsage  event.Reason = "AddOwnerRefToUsage"
	reasonAddInUseLabel    event.Reason = "AddInUseLabel"
	reasonRemoveInUseLabel event.Reason = "RemoveInUseLabel"
	reasonAddFinalizer     event.Reason = "AddFinalizer"
	reasonRemoveFinalizer  event.Reason = "RemoveFinalizer"
	reasonReplayDeletion   event.Reason = "ReplayDeletion"

	reasonUsageConfigured event.Reason = "UsageConfigured"
	reasonWaitUsing       event.Reason = "WaitingUsingDeleted"
)

// A SelectorResolver resolves a usage's resource selectors.
type SelectorResolver interface {
	ResolveSelectors(ctx context.Context, u protection.Usage) error
}

// SetupUsage adds a controller that reconciles Usages.
func SetupUsage(mgr ctrl.Manager, f Finder, o controller.Options) error {
	name := "usage/" + strings.ToLower(v1beta1.UsageGroupKind)
	r := NewReconciler(mgr,
		func() protection.Usage { return &protection.InternalUsage{} },
		f,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithPollInterval(o.PollInterval))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Usage{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupClusterUsage adds a controller that reconciles ClusterUsages.
func SetupClusterUsage(mgr ctrl.Manager, f Finder, o controller.Options) error {
	name := "usage/" + strings.ToLower(v1beta1.ClusterUsageGroupKind)
	r := NewReconciler(mgr,
		func() protection.Usage { return &protection.InternalClusterUsage{} },
		f,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithPollInterval(o.PollInterval))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.ClusterUsage{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupLegacyUsage adds a controller that reconciles legacy Usages, i.e. those
// in the apiextensions.crossplane.io API group.
func SetupLegacyUsage(mgr ctrl.Manager, f Finder, o controller.Options) error {
	name := "usage/" + strings.ToLower(legacy.UsageGroupKind)
	r := NewReconciler(mgr,
		func() protection.Usage { return &protection.InternalLegacyUsage{} },
		f,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithPollInterval(o.PollInterval))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&legacy.Usage{}). //nolint:staticcheck // It's deprecated, but we still need to support it.
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
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

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(c xpresource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = c
	}
}

// WithFinalizer specifies how the Reconciler should add and remove
// finalizers to and from the managed resource.
func WithFinalizer(f xpresource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.usage.Finalizer = f
	}
}

// WithSelectorResolver specifies how the Reconciler should resolve any
// resource references it encounters while reconciling Usages.
func WithSelectorResolver(sr SelectorResolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.usage.SelectorResolver = sr
	}
}

// WithPollInterval specifies how long the Reconciler should wait before queueing
// a new reconciliation after a successful reconcile. The Reconciler requeues
// after a specified duration when it is not actively waiting for an external
// operation, but wishes to check whether resources it does not have a watch on
// (i.e. used/using resources) need to be reconciled.
func WithPollInterval(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = after
	}
}

// A Finder finds usages.
type Finder interface {
	FindUsageOf(ctx context.Context, o usage.Object) ([]protection.Usage, error)
}

type usageResource struct {
	xpresource.Finalizer
	SelectorResolver
}

// NewReconciler returns a Reconciler for a type that implements the
// protection.Usage interface.
func NewReconciler(mgr manager.Manager, u func() protection.Usage, f Finder, opts ...ReconcilerOption) *Reconciler {
	// TODO(negz): Stop using this wrapper? It's only necessary if the client is
	// backed by a cache, and at the time of writing the manager's client isn't.
	// It's configured not to automatically cache unstructured objects. The
	// wrapper is needed when caching because controller-runtime doesn't support
	// caching types that satisfy runtime.Unstructured - it only supports the
	// concrete *unstructured.Unstructured type.
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		client: xpresource.ClientApplicator{
			Client:     kube,
			Applicator: xpresource.NewAPIUpdatingApplicator(kube),
		},

		newUsage: u,

		usage: usageResource{
			Finalizer:        xpresource.NewAPIFinalizer(kube, finalizer),
			SelectorResolver: newAPISelectorResolver(kube),
		},
		resource: f,

		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles Usages.
type Reconciler struct {
	client xpresource.ClientApplicator

	newUsage func() protection.Usage

	usage    usageResource
	resource Finder

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	pollInterval time.Duration
}

// Reconcile a Usage resource by resolving its selectors, defining ownership
// relationship, adding a finalizer and handling proper deletion.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are typically complex.
	log := r.log.WithValues("request", req)

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	u := r.newUsage()
	uu := u.Unwrap()
	if err := r.client.Get(ctx, req.NamespacedName, uu); err != nil {
		log.Debug(errGetUsage, "error", err)
		return reconcile.Result{}, errors.Wrap(xpresource.IgnoreNotFound(err), errGetUsage)
	}

	of := u.GetUserOf()
	by := u.GetUsedBy()

	// Validate APIVersion of used object provided as input.
	// We parse this value while indexing the objects, and we need to make sure it is valid.
	if _, err := schema.ParseGroupVersion(of.APIVersion); err != nil {
		return reconcile.Result{}, errors.Wrap(err, errParseAPIVersion)
	}

	orig := uu.DeepCopyObject()

	if err := r.usage.ResolveSelectors(ctx, u); err != nil {
		log.Debug(errResolveSelectors, "error", err)
		err = errors.Wrap(err, errResolveSelectors)
		r.record.Event(uu, event.Warning(reasonResolveSelectors, err))

		return reconcile.Result{}, err
	}

	status := r.conditions.For(uu)

	// Identify used as an unstructured object. It might not actually be
	// composed by an XR; we use composed as a convenience.
	used := composed.New(composed.FromReference(v1.ObjectReference{
		Kind:       of.Kind,
		Namespace:  ptr.Deref(of.ResourceRef.Namespace, uu.GetNamespace()),
		Name:       of.ResourceRef.Name,
		APIVersion: of.APIVersion,
	}))

	if meta.WasDeleted(uu) {
		// Note (turkenh): A Usage can be composed as part of a Composition
		// together with the using resource. When the composite is deleted, the
		// usage resource will also be deleted. In this case, we need to wait
		// for the using resource to be deleted; otherwise, we won’t be properly
		// waiting for its deletion since Usage will be gone immediately. We
		// intentionally avoid checking whether they are part of the same
		// Composition, as they may not be, but could still be composed together
		// as part of a higher-level Composition. Therefore, as an approximation
		// we wait for the using resource to be deleted before deleting the
		// usage resource, if the usage resource is part of a Composition.
		if by != nil && uu.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] != "" {
			// Identify using resource as an unstructured object.
			using := composed.New(composed.FromReference(v1.ObjectReference{
				Kind:       by.Kind,
				Namespace:  uu.GetNamespace(), // Will always be cluster scoped or in the same namespace as the Usage.
				Name:       by.ResourceRef.Name,
				APIVersion: by.APIVersion,
			}))
			// Get the using resource
			if err := r.client.Get(ctx, client.ObjectKeyFromObject(using), using); xpresource.IgnoreNotFound(err) != nil {
				log.Debug(errGetUsing, "error", err)
				err = errors.Wrap(xpresource.IgnoreNotFound(err), errGetUsing)
				r.record.Event(uu, event.Warning(reasonGetUsing, err))

				return reconcile.Result{}, err
			} else if err == nil {
				// Using resource is still there, so we need to wait for it to be deleted.
				msg := fmt.Sprintf("Waiting for the using resource (which is a %q named %q) to be deleted.", by.Kind, by.ResourceRef.Name)
				log.Debug(msg)
				r.record.Event(uu, event.Normal(reasonWaitUsing, msg))
				// We are using a waitPollInterval which is shorter than the
				// pollInterval to make sure we delete the usage as soon as
				// possible after the using resource is deleted. This is
				// to add minimal delay to the overall deletion process which is
				// usually extended by backoff intervals.
				return reconcile.Result{RequeueAfter: waitPollInterval}, nil
			}
		}

		// At this point using resource is either:
		// - not defined
		// - not found (e.g. deleted)
		// So, we can proceed with the deletion of the usage.

		// Get the used resource
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(used), used); xpresource.IgnoreNotFound(err) != nil {
			log.Debug(errGetUsed, "error", err)
			err = errors.Wrap(err, errGetUsed)
			r.record.Event(uu, event.Warning(reasonGetUsed, err))

			return reconcile.Result{}, err
		} else if err == nil {
			// Remove the in-use label from the used resource if no other usages exists.
			usages, err := r.resource.FindUsageOf(ctx, used)
			if err != nil {
				log.Debug(errFindUsages, "error", err)
				err = errors.Wrap(err, errFindUsages)
				r.record.Event(uu, event.Warning(reasonFindUsages, err))

				return reconcile.Result{}, err
			}
			// There are no "other" usageResource's referencing the used resource,
			// so we can remove the in-use label from the used resource
			if len(usages) < 2 {
				meta.RemoveLabels(used, inUseLabelKey)

				if err = r.client.Update(ctx, used); err != nil {
					log.Debug(errRemoveInUseLabel, "error", err)

					if kerrors.IsConflict(err) {
						return reconcile.Result{Requeue: true}, nil
					}

					err = errors.Wrap(err, errRemoveInUseLabel)
					r.record.Event(uu, event.Warning(reasonRemoveInUseLabel, err))

					return reconcile.Result{}, err
				}
			}
		}

		if ptr.Deref(u.GetReplayDeletion(), false) && used.GetAnnotations() != nil {
			if policy, ok := used.GetAnnotations()[protection.AnnotationKeyDeletionAttempt]; ok {
				// We have already recorded a deletion attempt and want to replay deletion, let's delete the used resource.

				//nolint:contextcheck // We cannot use the context from the reconcile function since it will be cancelled after the reconciliation.
				go func() {
					// We do the deletion async and after some delay to make sure the usage is deleted before the
					// deletion attempt. We remove the finalizer on this Usage right below, so, we know it will disappear
					// very soon.
					time.Sleep(2 * time.Second)
					log.Info("Replaying deletion of the used resource", "apiVersion", used.GetAPIVersion(), "kind", used.GetKind(), "name", used.GetName(), "policy", policy)

					if err := r.client.Delete(context.Background(), used, client.PropagationPolicy(policy)); err != nil {
						log.Info("Error when replaying deletion of the used resource", "apiVersion", used.GetAPIVersion(), "kind", used.GetKind(), "name", used.GetName(), "err", err)
					}
				}()
			}
		}

		// Remove the finalizer from the usage
		if err := r.usage.RemoveFinalizer(ctx, uu); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)

			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(uu, event.Warning(reasonRemoveFinalizer, err))

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Add finalizer for Usage resource.
	if err := r.usage.AddFinalizer(ctx, uu); err != nil {
		log.Debug(errAddFinalizer, "error", err)

		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(uu, event.Warning(reasonAddFinalizer, err))

		return reconcile.Result{}, err
	}

	d := detailsAnnotation(u)
	if uu.GetAnnotations()[detailsAnnotationKey] != d {
		meta.AddAnnotations(uu, map[string]string{
			detailsAnnotationKey: d,
		})

		if err := r.client.Update(ctx, uu); err != nil {
			log.Debug(errAddDetailsAnnotation, "error", err)

			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errAddDetailsAnnotation)
			r.record.Event(uu, event.Warning(reasonDetailsToUsage, err))

			return reconcile.Result{}, err
		}
	}

	// Get the used resource
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(used), used); err != nil {
		log.Debug(errGetUsed, "error", err)
		err = errors.Wrap(err, errGetUsed)
		r.record.Event(uu, event.Warning(reasonGetUsed, err))

		return reconcile.Result{}, err
	}

	// Used resource should have in-use label.
	if used.GetLabels()[inUseLabelKey] != "true" || !used.OwnedBy(uu.GetUID()) {
		// Note(turkenh): Composite controller will not remove this label with
		// new reconciles since it uses a patching applicator to update the
		// resource.
		meta.AddLabels(used, map[string]string{inUseLabelKey: "true"})

		if err := r.client.Update(ctx, used); err != nil {
			log.Debug(errAddInUseLabel, "error", err)

			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errAddInUseLabel)
			r.record.Event(uu, event.Warning(reasonAddInUseLabel, err))

			return reconcile.Result{}, err
		}
	}

	if by != nil {
		// Identify using resource as an unstructured object.
		using := composed.New(composed.FromReference(v1.ObjectReference{
			Kind:       by.Kind,
			Namespace:  uu.GetNamespace(), // Will always be cluster scoped or in the same namespace as the Usage.
			Name:       by.ResourceRef.Name,
			APIVersion: by.APIVersion,
		}))

		// Get the using resource
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(using), using); err != nil {
			log.Debug(errGetUsing, "error", err)
			err = errors.Wrap(err, errGetUsing)
			r.record.Event(uu, event.Warning(reasonGetUsing, err))

			return reconcile.Result{}, err
		}

		// usageResource should have a finalizer and be owned by the using resource.
		if owners := uu.GetOwnerReferences(); len(owners) == 0 || owners[0].UID != using.GetUID() {
			meta.AddOwnerReference(uu, meta.AsOwner(
				meta.TypedReferenceTo(using, using.GetObjectKind().GroupVersionKind()),
			))

			if err := r.client.Update(ctx, uu); err != nil {
				log.Debug(errAddOwnerToUsage, "error", err)

				if kerrors.IsConflict(err) {
					return reconcile.Result{Requeue: true}, nil
				}

				err = errors.Wrap(err, errAddOwnerToUsage)
				r.record.Event(uu, event.Warning(reasonOwnerRefToUsage, err))

				return reconcile.Result{}, err
			}
		}
	}

	status.MarkConditions(xpv1.Available())

	// We are only watching the Usage itself but not using or used resources.
	// So, we need to reconcile the Usage periodically to check if the using
	// or used resources are still there.
	if !cmp.Equal(uu, orig) {
		r.record.Event(uu, event.Normal(reasonUsageConfigured, "Usage configured successfully."))
		return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, uu), errUpdateStatus)
	}

	return reconcile.Result{RequeueAfter: r.pollInterval}, nil
}

func detailsAnnotation(u protection.Usage) string {
	if r := ptr.Deref(u.GetReason(), ""); r != "" {
		return r
	}

	by := u.GetUsedBy()

	of := u.GetUserOf()
	if by != nil {
		if ns := ptr.Deref(of.ResourceRef.Namespace, ""); ns != "" {
			return fmt.Sprintf("%s/%s uses %s/%s in namespace %s", by.Kind, by.ResourceRef.Name, of.Kind, of.ResourceRef.Name, ns)
		}

		return fmt.Sprintf("%s/%s uses %s/%s", by.Kind, by.ResourceRef.Name, of.Kind, of.ResourceRef.Name)
	}

	return "undefined"
}

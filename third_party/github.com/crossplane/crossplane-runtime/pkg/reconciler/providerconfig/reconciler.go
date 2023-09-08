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

// Package providerconfig provides a reconciler that manages the lifecycle of a
// ProviderConfig.
package providerconfig

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	finalizer = "in-use.crossplane.io"
	shortWait = 30 * time.Second
	timeout   = 2 * time.Minute

	errGetPC        = "cannot get ProviderConfig"
	errListPCUs     = "cannot list ProviderConfigUsages"
	errDeletePCU    = "cannot delete ProviderConfigUsage"
	errUpdate       = "cannot update ProviderConfig"
	errUpdateStatus = "cannot update ProviderConfig status"
)

// Event reasons.
const (
	reasonAccount event.Reason = "UsageAccounting"
)

// Condition types and reasons.
const (
	TypeTerminating xpv1.ConditionType   = "Terminating"
	ReasonInUse     xpv1.ConditionReason = "InUse"
)

// Terminating indicates a ProviderConfig has been deleted, but that the
// deletion is being blocked because it is still in use.
func Terminating() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeTerminating,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInUse,
	}
}

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of managed resource.
func ControllerName(kind string) string {
	return "providerconfig/" + strings.ToLower(kind)
}

// A Reconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type Reconciler struct {
	client client.Client

	newConfig    func() resource.ProviderConfig
	newUsageList func() resource.ProviderConfigUsageList

	log    logging.Logger
	record event.Recorder
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

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

// NewReconciler returns a Reconciler of ProviderConfigs.
func NewReconciler(m manager.Manager, of resource.ProviderConfigKinds, o ...ReconcilerOption) *Reconciler {
	nc := func() resource.ProviderConfig {
		return resource.MustCreateObject(of.Config, m.GetScheme()).(resource.ProviderConfig)
	}
	nul := func() resource.ProviderConfigUsageList {
		return resource.MustCreateObject(of.UsageList, m.GetScheme()).(resource.ProviderConfigUsageList)
	}

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_, _ = nc(), nul()

	r := &Reconciler{
		client: m.GetClient(),

		newConfig:    nc,
		newUsageList: nul,

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a ProviderConfig by accounting for the managed resources that are
// using it, and ensuring it cannot be deleted until it is no longer in use.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pc := r.newConfig()
	if err := r.client.Get(ctx, req.NamespacedName, pc); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetPC, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPC)
	}

	log = log.WithValues(
		"uid", pc.GetUID(),
		"version", pc.GetResourceVersion(),
		"name", pc.GetName(),
	)

	l := r.newUsageList()
	if err := r.client.List(ctx, l, client.MatchingLabels{xpv1.LabelKeyProviderName: pc.GetName()}); err != nil {
		log.Debug(errListPCUs, "error", err)
		r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errListPCUs)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	users := int64(len(l.GetItems()))
	for _, pcu := range l.GetItems() {
		if metav1.GetControllerOf(pcu) == nil {
			// Usages should always have a controller reference. If this one has
			// none it's probably been stripped off (e.g. by a Velero restore).
			// We can safely delete it - it's either stale, or will be recreated
			// next time the relevant managed resource connects.
			if err := r.client.Delete(ctx, pcu); resource.IgnoreNotFound(err) != nil {
				log.Debug(errDeletePCU, "error", err)
				r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errDeletePCU)))
				return reconcile.Result{RequeueAfter: shortWait}, nil //nolint:nilerr // Returning err would make us requeue instantly.
			}
			users--
		}
	}
	log = log.WithValues("usages", users)

	if meta.WasDeleted(pc) {
		if users > 0 {
			msg := "Blocking deletion while usages still exist"

			log.Debug(msg)
			r.record.Event(pc, event.Warning(reasonAccount, errors.New(msg)))

			// We're watching our usages, so we'll be requeued when they go.
			pc.SetUsers(users)
			pc.SetConditions(Terminating().WithMessage(msg))
			return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pc), errUpdateStatus)
		}

		meta.RemoveFinalizer(pc, finalizer)
		if err := r.client.Update(ctx, pc); err != nil {
			r.log.Debug(errUpdate, "error", err)
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		// We've been deleted - there's no more work to do.
		return reconcile.Result{Requeue: false}, nil
	}

	meta.AddFinalizer(pc, finalizer)
	if err := r.client.Update(ctx, pc); err != nil {
		r.log.Debug(errUpdate, "error", err)
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// There's no need to requeue explicitly - we're watching all PCs.
	pc.SetUsers(users)
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pc), errUpdateStatus)
}

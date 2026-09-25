/*
Copyright 2025 The Crossplane Authors.

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

package discovery

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

const (
	reasonDiscovery      = "DiscoveryReconciliation"
	reasonListFailed     = "ExternalListFailed"
	reasonQueryFailed    = "ManagedResourceQueryFailed"
	reasonUpdateFailed   = "DiscoveryReportUpdateFailed"
	reasonDiscovered     = "ResourcesDiscovered"
	reasonDiscoveryError = "DiscoveryError"
)

// A Reconciler reconciles DiscoveryReports by discovering external resources.
type Reconciler struct {
	client  client.Client
	dynamic dynamic.Interface
	log     logging.Logger
	record  event.Recorder
}

// NewReconciler returns a new Reconciler.
func NewReconciler(mgr ctrl.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:  mgr.GetClient(),
		dynamic: dynamic.NewForConfigOrDie(mgr.GetConfig()),
		log:     logging.NewNopLogger(),
		record:  event.NewNopRecorder(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Reconcile discovers external resources for a DiscoveryReport.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)

	dr := &v1alpha1.DiscoveryReport{}
	if err := r.client.Get(ctx, req.NamespacedName, dr); err != nil {
		log.Debug("cannot get DiscoveryReport", "error", err)
		return reconcile.Result{}, errors.Wrap(client.IgnoreNotFound(err), "cannot get DiscoveryReport")
	}

	// Initialize status if needed
	if dr.Status.Conditions == nil {
		dr.Status.Conditions = make([]xpv2.Condition, 0)
	}

	// Look up the ManagedResourceDefinition
	mrd := &v1alpha1.ManagedResourceDefinition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: dr.Spec.MRDRef.Name}, mrd); err != nil {
		log.Debug("cannot get ManagedResourceDefinition", "error", err)
		r.record.Event(dr, event.Warning(reasonDiscoveryError, err))
		dr.Status.Conditions = setCondition(dr.Status.Conditions, xpv2.Unavailable().WithMessage("cannot get ManagedResourceDefinition"))
		_ = r.client.Status().Update(ctx, dr)
		return reconcile.Result{}, errors.Wrap(client.IgnoreNotFound(err), "cannot get ManagedResourceDefinition")
	}

	// TODO: Get provider's ExternalClient and check if it implements ExternalLister
	// For now, return error saying this is not yet implemented
	r.record.Event(dr, event.Warning(reasonDiscoveryError, errors.New("provider discovery integration not yet implemented")))
	dr.Status.Conditions = setCondition(dr.Status.Conditions, xpv2.Unavailable().WithMessage("provider discovery integration not yet implemented"))
	_ = r.client.Status().Update(ctx, dr)

	// Schedule next reconcile at configured interval
	interval := 1 * time.Hour
	if dr.Spec.Interval != (metav1.Duration{}) {
		interval = dr.Spec.Interval.Duration
	}

	return reconcile.Result{RequeueAfter: interval}, nil
}

// setCondition sets the given condition in the conditions list, replacing any
// existing condition of the same type.
func setCondition(conditions []xpv2.Condition, c xpv2.Condition) []xpv2.Condition {
	if conditions == nil {
		conditions = []xpv2.Condition{}
	}

	// Check if condition of this type already exists
	for i, existing := range conditions {
		if existing.Type == c.Type {
			conditions[i] = c
			return conditions
		}
	}

	// Condition doesn't exist, append it
	return append(conditions, c)
}

// listExternal lists all external resources of the kind defined in the MRD.
// This is a placeholder that will call the provider's ExternalLister.
func (r *Reconciler) listExternal(ctx context.Context, mrd *v1alpha1.ManagedResourceDefinition) ([]v1alpha1.ExternalResource, error) {
	_ = ctx
	_ = mrd

	// TODO: Implement actual external resource listing via provider's ExternalLister
	// For now, return empty list
	return []v1alpha1.ExternalResource{}, nil
}

// getExistingManagedResources queries the cluster for all managed resources of the given kind.
func (r *Reconciler) getExistingManagedResources(ctx context.Context, mrd *v1alpha1.ManagedResourceDefinition) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    mrd.Spec.Group,
		Version:  "v1",
		Resource: "buckets", // TODO: Pluralize from MRD.Spec.Names.Kind
	}

	list, err := r.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list managed resources")
	}

	var names []string
	for _, item := range list.Items {
		extName, found, _ := unstructured.NestedString(item.Object, "metadata", "annotations", "crossplane.io/external-name")
		if found && extName != "" {
			names = append(names, extName)
		}
	}

	return names, nil
}

// computeDifference returns external resources that are not managed by Crossplane.
func computeDifference(external []v1alpha1.ExternalResource, existing []string) []v1alpha1.ExternalResource {
	existingSet := make(map[string]bool)
	for _, name := range existing {
		existingSet[name] = true
	}

	unmanaged := []v1alpha1.ExternalResource{}
	for _, res := range external {
		if !existingSet[res.ExternalName] {
			res.LastSeen = &metav1.Time{Time: time.Now()}
			unmanaged = append(unmanaged, res)
		}
	}

	return unmanaged
}

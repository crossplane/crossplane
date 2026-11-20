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
	"fmt"
	"strings"
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

	// Find ProviderConfigs for this MRD's provider
	log.Debug("Discovering external resources", "mrd", mrd.Name, "provider", mrd.Spec.Group)

	// If a specific ProviderConfig is requested, use only that one
	var providerConfigs []string
	if dr.Spec.ProviderConfig != nil {
		providerConfigs = append(providerConfigs, dr.Spec.ProviderConfig.Name)
	} else {
		// Otherwise, discover from all ProviderConfigs of this provider (not yet implemented)
		log.Debug("All ProviderConfigs discovery not yet implemented, specify providerConfig in spec")
	}

	// Discover external resources for each ProviderConfig
	var allExternal []v1alpha1.ExternalResource
	var discoveryErrors []string

	for _, pcName := range providerConfigs {
		external, err := r.discoverExternal(ctx, log, mrd, pcName)
		if err != nil {
			log.Debug("discovery failed for ProviderConfig", "pcName", pcName, "error", err)
			discoveryErrors = append(discoveryErrors, fmt.Sprintf("%s: %v", pcName, err))
			continue
		}
		allExternal = append(allExternal, external...)
	}

	// If discovery failed, mark status and requeue
	if len(providerConfigs) > 0 && len(allExternal) == 0 && len(discoveryErrors) > 0 {
		msg := "discovery failed: " + strings.Join(discoveryErrors, "; ")
		r.record.Event(dr, event.Warning(reasonDiscoveryError, errors.New(msg)))
		dr.Status.Conditions = setCondition(dr.Status.Conditions, xpv2.Unavailable().WithMessage(msg))
		_ = r.client.Status().Update(ctx, dr)
		// Requeue after a shorter interval on error
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Get existing managed resources in cluster
	existing, err := r.getExistingManagedResources(ctx, mrd)
	if err != nil {
		log.Debug("cannot query existing managed resources", "error", err)
		r.record.Event(dr, event.Warning(reasonQueryFailed, err))
		dr.Status.Conditions = setCondition(dr.Status.Conditions, xpv2.Unavailable().WithMessage("cannot query managed resources"))
		_ = r.client.Status().Update(ctx, dr)
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Compute unmanaged resources (external - existing)
	unmanaged := computeDifference(allExternal, existing)

	// Update DiscoveryReport status
	now := metav1.Now()
	discoveredCount := int64(len(allExternal))
	unmanagedCount := int64(len(unmanaged))

	dr.Status.DiscoveredResourceCount = &discoveredCount
	dr.Status.UnmanagedResourceCount = &unmanagedCount
	dr.Status.UnmanagedResources = unmanaged
	dr.Status.LastDiscoveryTime = &now

	// Set success condition
	dr.Status.Conditions = setCondition(dr.Status.Conditions, xpv2.Available().WithMessage(
		fmt.Sprintf("Discovered %d resources, %d unmanaged", discoveredCount, unmanagedCount)))

	// Update status in cluster
	if err := r.client.Status().Update(ctx, dr); err != nil {
		log.Debug("cannot update DiscoveryReport status", "error", err)
		r.record.Event(dr, event.Warning(reasonUpdateFailed, err))
		return reconcile.Result{}, errors.Wrap(err, "cannot update DiscoveryReport status")
	}

	r.record.Event(dr, event.Normal(reasonDiscovered, fmt.Sprintf("Discovered %d unmanaged resources", unmanagedCount)))

	// Schedule next reconcile at configured interval
	interval := 1 * time.Hour
	if dr.Spec.Interval != (metav1.Duration{}) {
		interval = dr.Spec.Interval.Duration
	}
	nextTime := metav1.NewTime(time.Now().Add(interval))
	dr.Status.NextDiscoveryTime = &nextTime

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

// discoverExternal returns the external resources that the provider published
// for the MRD in its ExternalInventory. Crossplane does not call providers:
// providers list their resources and write the inventory.
//
// The inventory must have been listed with the given ProviderConfig. If any
// scope failed to list, the result is an error, so an incomplete listing is
// never reported as complete.
func (r *Reconciler) discoverExternal(ctx context.Context, log logging.Logger, mrd *v1alpha1.ManagedResourceDefinition, pcName string) ([]v1alpha1.ExternalResource, error) {
	inv := &v1alpha1.ExternalInventory{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: mrd.Name}, inv); err != nil {
		return nil, errors.Wrapf(err, "cannot get ExternalInventory for ManagedResourceDefinition %q; the provider has not published a listing", mrd.Name)
	}

	if ref := inv.Spec.ProviderConfigRef; ref != nil && ref.Name != pcName {
		return nil, errors.Errorf("ExternalInventory %q was listed with ProviderConfig %q, not %q", mrd.Name, ref.Name, pcName)
	}

	var (
		out      []v1alpha1.ExternalResource
		failures []string
	)
	for _, sc := range inv.Status.Scopes {
		if sc.Error != "" {
			failures = append(failures, fmt.Sprintf("scope %v: %s", sc.Scope, sc.Error))
			continue
		}

		for _, item := range sc.Items {
			out = append(out, v1alpha1.ExternalResource{ExternalName: item.ExternalName, Scope: item.Scope})
		}
	}

	if len(failures) > 0 {
		return nil, errors.Errorf("%d of %d scopes failed to list: %s", len(failures), len(inv.Status.Scopes), strings.Join(failures, "; "))
	}

	log.Debug("Read ExternalInventory", "mrd", mrd.Name, "resources", len(out))

	return out, nil
}

// existingManagedResource is a managed resource of the kind being discovered:
// its external name and the spec.forProvider fields that scope it.
type existingManagedResource struct {
	ExternalName string
	ForProvider  map[string]any
}

// getExistingManagedResources queries the cluster for all managed resources of the given kind.
func (r *Reconciler) getExistingManagedResources(ctx context.Context, mrd *v1alpha1.ManagedResourceDefinition) ([]existingManagedResource, error) {
	// Pluralize the resource name (simple heuristic: add 's')
	// For full correctness, this should use the MRD's plural name or APIGroup discovery
	plural := strings.ToLower(mrd.Spec.Names.Kind) + "s"

	gvr := schema.GroupVersionResource{
		Group:    mrd.Spec.Group,
		Version:  "v1",
		Resource: plural,
	}

	// Try to list cluster-scoped resources first (no namespace)
	list, err := r.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If that fails, it might be namespaced. For now, return error.
		// A production implementation would handle namespaced resources.
		return nil, errors.Wrap(err, "cannot list managed resources")
	}

	var existing []existingManagedResource
	for _, item := range list.Items {
		// Extract external-name annotation which identifies the external resource
		extName, found, _ := unstructured.NestedString(item.Object, "metadata", "annotations", "crossplane.io/external-name")
		if !found || extName == "" {
			continue
		}

		forProvider, _, _ := unstructured.NestedMap(item.Object, "spec", "forProvider")
		existing = append(existing, existingManagedResource{ExternalName: extName, ForProvider: forProvider})
	}

	return existing, nil
}

// computeDifference returns external resources that are not managed by Crossplane.
// A resource is managed when a managed resource has its external name and the
// same value for every scope field.
func computeDifference(external []v1alpha1.ExternalResource, existing []existingManagedResource) []v1alpha1.ExternalResource {
	unmanaged := []v1alpha1.ExternalResource{}
	for _, res := range external {
		if !isManaged(res, existing) {
			res.LastSeen = &metav1.Time{Time: time.Now()}
			unmanaged = append(unmanaged, res)
		}
	}

	return unmanaged
}

// isManaged reports whether res matches a managed resource by identity.
func isManaged(res v1alpha1.ExternalResource, existing []existingManagedResource) bool {
	for _, e := range existing {
		if e.ExternalName == res.ExternalName && scopeMatches(e.ForProvider, res.Scope) {
			return true
		}
	}

	return false
}

// scopeMatches reports whether forProvider has the scope's value for every
// scope field.
func scopeMatches(forProvider map[string]any, scope map[string]string) bool {
	for k, want := range scope {
		got, ok := forProvider[k]
		if !ok || fmt.Sprint(got) != want {
			return false
		}
	}

	return true
}

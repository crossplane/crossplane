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

// discoverExternal calls the provider's ExternalLister to discover external resources.
// This method handles the provider integration and pagination.
func (r *Reconciler) discoverExternal(ctx context.Context, log logging.Logger, mrd *v1alpha1.ManagedResourceDefinition, pcName string) ([]v1alpha1.ExternalResource, error) {
	// PROVIDER INTEGRATION POINT:
	// This is where we would call the provider's ExternalLister interface.
	// The provider must be instantiated (typically via the provider's own controller)
	// and implement the ExternalLister interface defined in crossplane-runtime.
	//
	// Current limitations:
	// 1. Providers run as separate Deployments with their own controllers
	// 2. The ExternalClient is instantiated per ProviderConfig in the provider pod
	// 3. Crossplane discovery controller cannot directly import or instantiate providers
	//
	// Solutions for future implementation:
	// A. Provider webhook/gRPC endpoint: Provider exposes an endpoint for discovery
	// B. Provider controller populates CR: Provider watches MRD and populates CR with results
	// C. Direct provider integration: Crossplane imports provider and calls directly
	//
	// For now, return an appropriate error message indicating provider integration needed.

	log.Debug("external resource discovery requires provider implementation",
		"provider", mrd.Spec.Group, "providerConfig", pcName)

	// Placeholder: This would be replaced with actual provider integration
	// var lister resource.ExternalLister
	// if ec, ok := provider.(resource.ExternalLister); ok {
	//     return r.listWithPagination(ctx, ec, pageToken)
	// }

	return []v1alpha1.ExternalResource{}, errors.New(
		"external resource discovery requires provider implementation of ExternalLister interface")
}

// listWithPagination handles paginated results from ExternalLister.
// This is a helper for when providers implement discovery.
func (r *Reconciler) listWithPagination(ctx context.Context, pageToken string, listFunc func(ctx context.Context, token string) ([]string, string, error)) ([]v1alpha1.ExternalResource, error) {
	var resources []v1alpha1.ExternalResource

	for {
		names, nextToken, err := listFunc(ctx, pageToken)
		if err != nil {
			return nil, errors.Wrap(err, "cannot list external resources")
		}

		for _, name := range names {
			resources = append(resources, v1alpha1.ExternalResource{
				ExternalName: name,
			})
		}

		// Stop if no more pages
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}

	return resources, nil
}

// getExistingManagedResources queries the cluster for all managed resources of the given kind.
func (r *Reconciler) getExistingManagedResources(ctx context.Context, mrd *v1alpha1.ManagedResourceDefinition) ([]string, error) {
	// Pluralize the resource name (simple heuristic: add 's')
	// For full correctness, this should use the MRD's plural name or APIGroup discovery
	plural := strings.ToLower(mrd.Spec.Names.Kind) + "s"

	gvr := schema.GroupVersionResource{
		Group:    mrd.Spec.Group,
		Version:  mrd.Spec.Names.Categories[0], // Use first category as fallback, typically "v1"
		Resource: plural,
	}

	// Try to list cluster-scoped resources first (no namespace)
	list, err := r.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If that fails, it might be namespaced. For now, return error.
		// A production implementation would handle namespaced resources.
		return nil, errors.Wrap(err, "cannot list managed resources")
	}

	var names []string
	for _, item := range list.Items {
		// Extract external-name annotation which identifies the external resource
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

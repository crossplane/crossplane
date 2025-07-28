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

// Package watched implements a controller for resources watched by WatchOperations.
package watched

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	"github.com/crossplane/crossplane/internal/ops/lifecycle"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonListOperations          event.Reason = "ListOperations"
	reasonReplaceRunningOperation event.Reason = "ReplaceRunningOperation"
	reasonCreateOperation         event.Reason = "CreateOperation"
	reasonWatchOperationGet       event.Reason = "GetWatchOperation"
)

// A Reconciler reconciles watched resources by creating Operations
// when the watched resources change.
type Reconciler struct {
	client client.Client
	log    logging.Logger
	record event.Recorder

	watchOpName string
	watchedGVK  schema.GroupVersionKind
}

// Reconcile is triggered when a watched resource changes, and creates an
// Operation using the WatchOperation's template.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues(
		"request", req,
		"gvk", r.watchedGVK,
	)
	log.Debug("Reconciling watched resource")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get the watched resource that triggered this reconcile.
	watched := &unstructured.Unstructured{}
	watched.SetGroupVersionKind(r.watchedGVK)
	if err := r.client.Get(ctx, req.NamespacedName, watched); err != nil {
		if !kerrors.IsNotFound(err) {
			log.Debug("Cannot get watched resource", "error", err)
			return reconcile.Result{}, errors.Wrap(err, "cannot get watched resource")
		}

		// NOTE(negz): The underlying informer has the final snapshot of
		// the object when it was deleted, but controller-runtime does
		// not expose it to us (by design).
		//
		// In this case I'd rather have the final snapshot, but not so
		// much that I want to fight controller-runtime about it. The
		// only downside of using this synthetic resource is that if we
		// see the same delete event multiple times we'll create
		// multiple Operations (due to the different synthetic deletion
		// timestamps). This should be unlikely as the work queue tries
		// to dedupe
		//
		// We could also force access to the deleted event by adding a
		// finalizer to all watched resources, but that's way too
		// invasive for my taste.

		log.Debug("Watched resource was deleted, using synthetic resource to process deletion event")

		watched.SetName(req.Name)
		watched.SetNamespace(req.Namespace)
		watched.SetDeletionTimestamp(ptr.To(metav1.Now()))
		watched.SetResourceVersion(v1alpha1.SyntheticResourceVersionDeleted)
	}

	log = log.WithValues(
		"uid", watched.GetUID(),
		"version", watched.GetResourceVersion(),
		"namespace", watched.GetNamespace(),
		"name", watched.GetName(),
	)

	// Get the current WatchOperation to ensure it still exists and get latest spec.
	wo := &v1alpha1.WatchOperation{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: r.watchOpName}, wo); err != nil {
		log.Debug("Cannot get WatchOperation", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get WatchOperation")
	}

	// Don't reconcile if the WatchOperation is paused.
	if meta.IsPaused(wo) {
		log.Debug("WatchOperation is paused")
		return reconcile.Result{Requeue: false}, nil
	}

	// Don't reconcile if the WatchOperation is being deleted.
	if meta.WasDeleted(wo) {
		log.Debug("WatchOperation is being deleted")
		return reconcile.Result{Requeue: false}, nil
	}

	// List existing Operations for this WatchOperation.
	ol := &v1alpha1.OperationList{}
	if err := r.client.List(ctx, ol, client.MatchingLabels{v1alpha1.LabelWatchOperationName: wo.GetName()}); err != nil {
		log.Debug("Cannot list Operations", "error", err)
		err = errors.Wrap(err, "cannot list Operations")
		r.record.Event(wo, event.Warning(reasonListOperations, err))
		return reconcile.Result{}, err
	}

	// Record all running Operations.
	running := make(map[string]bool)
	for _, op := range lifecycle.WithReason(v1alpha1.ReasonPipelineRunning, ol.Items...) {
		running[op.GetName()] = true
	}

	// Check concurrency policy before creating new Operations.
	if len(running) > 0 {
		switch p := ptr.Deref(wo.Spec.ConcurrencyPolicy, v1alpha1.ConcurrencyPolicyAllow); p {
		case v1alpha1.ConcurrencyPolicyAllow:
			log.Debug("Concurrency policy allows creating Operations while other Operations are running", "policy", p, "running", len(running))
		case v1alpha1.ConcurrencyPolicyForbid:
			log.Debug("Concurrency policy forbids creating Operations while other Operations are running", "policy", p, "running", len(running))
			return reconcile.Result{Requeue: false}, nil
		case v1alpha1.ConcurrencyPolicyReplace:
			log.Debug("Concurrency policy requires deleting other running Operations", "policy", p, "running", len(running))
			for _, op := range ol.Items {
				if !running[op.GetName()] {
					continue
				}
				if err := r.client.Delete(ctx, &op); resource.IgnoreNotFound(err) != nil {
					log.Debug("Cannot delete running Operation", "error", err, "operation", op.GetName())
					err = errors.Wrapf(err, "cannot delete running Operation %q", op.GetName())
					r.record.Event(wo, event.Warning(reasonReplaceRunningOperation, err))
					return reconcile.Result{}, err
				}
				log.Debug("Deleted running operation due to concurrency policy", "policy", p, "operation", op.GetName())
			}
		}
	}

	// Generate a unique name for the Operation.
	name := OperationName(wo, watched)

	// Check if we've already created an Operation for this resource version.
	for _, op := range ol.Items {
		if op.GetName() == name {
			log.Debug("Operation already exists for this resource version", "operation", name)
			return reconcile.Result{}, nil
		}
	}

	// Create the Operation.
	op := NewOperation(wo, watched, name)
	if err := r.client.Create(ctx, op); err != nil {
		log.Debug("Cannot create Operation", "error", err, "operation", op.GetName())
		err = errors.Wrapf(err, "cannot create Operation %q", op.GetName())
		r.record.Event(wo, event.Warning(reasonCreateOperation, err))
		return reconcile.Result{}, err
	}

	log.Debug("Created Operation for watched resource", "operation", op.GetName(), "resource", watched.GetName())
	return reconcile.Result{}, nil
}

// OperationName generates a deterministic and unique name for an Operation
// based on the WatchOperation name and a hash of the watched resource's GVK,
// namespace, name, UID, resource version, and deletion timestamp.
func OperationName(wo *v1alpha1.WatchOperation, watched *unstructured.Unstructured) string {
	in := watched.GroupVersionKind().String() + "/" +
		watched.GetNamespace() + "/" +
		watched.GetName() + "/" +
		string(watched.GetUID()) + "/" +
		watched.GetResourceVersion()

	// For synthetic deletion events, a unique deletion timestamp is set to
	// ensure different resource instances (even with the same
	// name/namespace) create different Operation names.
	if t := watched.GetDeletionTimestamp(); t != nil {
		in = in + "/" + t.UTC().Format(time.RFC3339)
	}

	hash := sha256.Sum256([]byte(in))
	return wo.GetName() + "-" + hex.EncodeToString(hash[:])[:7]
}

// NewOperation creates a new Operation using the WatchOperation's template,
// injecting the watched resource into all pipeline steps.
func NewOperation(wo *v1alpha1.WatchOperation, watched *unstructured.Unstructured, name string) *v1alpha1.Operation {
	// Deep copy the spec to avoid mutating the original template
	spec := wo.Spec.OperationTemplate.Spec.DeepCopy()

	sel := v1alpha1.RequiredResourceSelector{
		RequirementName: v1alpha1.RequirementNameWatchedResource,
		APIVersion:      watched.GetAPIVersion(),
		Kind:            watched.GetKind(),
		Name:            ptr.To(watched.GetName()),
	}

	// Add namespace if the resource is namespaced
	if watched.GetNamespace() != "" {
		sel.Namespace = ptr.To(watched.GetNamespace())
	}

	// Inject the watched resource into each pipeline step
	for i := range spec.Pipeline {
		step := &spec.Pipeline[i]

		if step.Requirements == nil {
			step.Requirements = &v1alpha1.FunctionRequirements{}
		}

		step.Requirements.RequiredResources = append(step.Requirements.RequiredResources, sel)
	}

	op := &v1alpha1.Operation{
		ObjectMeta: wo.Spec.OperationTemplate.ObjectMeta,
		Spec:       *spec,
	}

	op.SetName(name)
	meta.AddLabels(op, map[string]string{v1alpha1.LabelWatchOperationName: wo.GetName()})

	// Add annotations with information about the watched resource
	annotations := map[string]string{
		v1alpha1.AnnotationWatchedResourceAPIVersion:      watched.GetAPIVersion(),
		v1alpha1.AnnotationWatchedResourceKind:            watched.GetKind(),
		v1alpha1.AnnotationWatchedResourceName:            watched.GetName(),
		v1alpha1.AnnotationWatchedResourceResourceVersion: watched.GetResourceVersion(),
	}

	// Add namespace annotation if the resource is namespaced
	if watched.GetNamespace() != "" {
		annotations[v1alpha1.AnnotationWatchedResourceNamespace] = watched.GetNamespace()
	}

	meta.AddAnnotations(op, annotations)

	av, k := v1alpha1.WatchOperationGroupVersionKind.ToAPIVersionAndKind()
	meta.AddOwnerReference(op, meta.AsController(&xpv1.TypedReference{
		APIVersion: av,
		Kind:       k,
		Name:       wo.GetName(),
		UID:        wo.GetUID(),
	}))

	return op
}

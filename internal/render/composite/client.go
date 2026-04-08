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

package composite

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// storeKey uniquely identifies a resource in the fake client's store.
type storeKey struct {
	schema.GroupVersionKind
	types.NamespacedName
}

// InMemoryClient is a fake client.Client that serves reads from an in-memory
// store and captures writes for later inspection. It is designed to back the
// real Crossplane XR reconciler during local render operations.
//
// It is not a general-purpose fake. It implements just enough of the
// client.Client interface to support the XR reconciler's access patterns.
type InMemoryClient struct {
	store  map[storeKey]unstructured.Unstructured
	scheme *runtime.Scheme

	// applied tracks resources written via SSA Patch (composed resources,
	// XR resourceRefs, XR status).
	applied []unstructured.Unstructured

	// deleted tracks resources removed via Delete (garbage collection).
	deleted []unstructured.Unstructured

	// updated tracks resources written via Update or Status().Update().
	// The last Status().Update for the XR is the final output.
	updated []unstructured.Unstructured
}

// NewInMemoryClient returns a new InMemoryClient pre-populated with the
// supplied resources. The scheme is used by Scheme() and
// GroupVersionKindFor().
func NewInMemoryClient(s *runtime.Scheme, resources ...unstructured.Unstructured) *InMemoryClient {
	store := make(map[storeKey]unstructured.Unstructured, len(resources))
	for _, r := range resources {
		key := storeKey{
			GroupVersionKind: r.GroupVersionKind(),
			NamespacedName: types.NamespacedName{
				Namespace: r.GetNamespace(),
				Name:      r.GetName(),
			},
		}
		store[key] = *r.DeepCopy()
	}

	return &InMemoryClient{
		store:  store,
		scheme: s,
	}
}

// Applied returns all resources that were written via SSA Patch calls.
func (c *InMemoryClient) Applied() []unstructured.Unstructured {
	out := make([]unstructured.Unstructured, len(c.applied))
	copy(out, c.applied)
	return out
}

// Deleted returns all resources that were removed via Delete calls.
func (c *InMemoryClient) Deleted() []unstructured.Unstructured {
	out := make([]unstructured.Unstructured, len(c.deleted))
	copy(out, c.deleted)
	return out
}

// Updated returns all resources that were written via Update or
// Status().Update() calls.
func (c *InMemoryClient) Updated() []unstructured.Unstructured {
	out := make([]unstructured.Unstructured, len(c.updated))
	copy(out, c.updated)
	return out
}

// Get retrieves a resource from the in-memory store.
func (c *InMemoryClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	sk := storeKey{GroupVersionKind: gvk, NamespacedName: key}

	stored, ok := c.store[sk]
	if !ok {
		return kerrors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind, // Not technically correct but sufficient.
		}, key.Name)
	}

	// Deep copy the stored resource into obj.
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Try to convert for typed objects.
		return runtime.DefaultUnstructuredConverter.FromUnstructured(stored.Object, obj)
	}

	stored.DeepCopyInto(u)
	return nil
}

// List lists resources from the in-memory store, filtering by GVK, namespace,
// and label selector.
func (c *InMemoryClient) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	listOpts := &client.ListOptions{}
	for _, o := range opts {
		o.ApplyToList(listOpts)
	}

	ul, ok := list.(*unstructured.UnstructuredList)
	if !ok {
		return kerrors.NewInternalError(nil)
	}

	gvk := ul.GroupVersionKind()
	// List GVKs are typically FooList; strip the "List" suffix for item matching.
	itemGVK := gvk
	if kind := gvk.Kind; len(kind) > 4 && kind[len(kind)-4:] == "List" {
		itemGVK.Kind = kind[:len(kind)-4]
	}

	items := make([]unstructured.Unstructured, 0, len(c.store))
	for key, r := range c.store {
		if key.Group != itemGVK.Group || key.Kind != itemGVK.Kind {
			continue
		}
		// Version matching: accept if either matches or one is empty.
		if itemGVK.Version != "" && key.Version != "" && key.Version != itemGVK.Version {
			continue
		}

		// Namespace filter.
		if listOpts.Namespace != "" && r.GetNamespace() != listOpts.Namespace {
			continue
		}

		// Label selector.
		if listOpts.LabelSelector != nil && !listOpts.LabelSelector.Matches(labels.Set(r.GetLabels())) {
			continue
		}

		items = append(items, *r.DeepCopy())
	}

	ul.Items = items
	return nil
}

// Create stores a new resource.
func (c *InMemoryClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)
	c.store[key] = *u
	return nil
}

// Update stores the updated resource and records it.
func (c *InMemoryClient) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)
	c.store[key] = *u
	c.updated = append(c.updated, *u.DeepCopy())
	return nil
}

// Patch handles SSA Apply patches by capturing the applied resource. For other
// patch types it behaves like Update.
func (c *InMemoryClient) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)

	if patch.Type() == types.ApplyPatchType {
		c.applied = append(c.applied, *u.DeepCopy())
		c.store[key] = *u
		return nil
	}

	c.store[key] = *u
	return nil
}

// Delete removes the resource from the store and records the deletion.
func (c *InMemoryClient) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)

	if stored, ok := c.store[key]; ok {
		c.deleted = append(c.deleted, *stored.DeepCopy())
		delete(c.store, key)
	}

	return nil
}

// DeleteAllOf is not used by the XR reconciler.
func (c *InMemoryClient) DeleteAllOf(_ context.Context, _ client.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}

// Apply is not used by the XR reconciler.
func (c *InMemoryClient) Apply(_ context.Context, _ runtime.ApplyConfiguration, _ ...client.ApplyOption) error {
	return nil
}

// Status returns a SubResourceWriter that handles status updates and patches
// by merging the incoming status into the stored resource's full state.
func (c *InMemoryClient) Status() client.SubResourceWriter {
	return &inMemoryStatusWriter{client: c}
}

// SubResource returns a SubResourceClient. The XR reconciler only uses
// Status(), not arbitrary subresources.
func (c *InMemoryClient) SubResource(_ string) client.SubResourceClient {
	return &inMemorySubResourceClient{client: c}
}

// Scheme returns the client's scheme.
func (c *InMemoryClient) Scheme() *runtime.Scheme {
	return c.scheme
}

// RESTMapper returns nil. The XR reconciler does not use the REST mapper
// directly.
func (c *InMemoryClient) RESTMapper() meta.RESTMapper {
	return nil
}

// GroupVersionKindFor returns the GVK of the given object.
func (c *InMemoryClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return obj.GetObjectKind().GroupVersionKind(), nil
}

// IsObjectNamespaced returns true by default. This is correct for most
// composed resources (managed resources are namespaced or this check is
// only relevant for namespaced XRs).
func (c *InMemoryClient) IsObjectNamespaced(_ runtime.Object) (bool, error) {
	return true, nil
}

// inMemoryStatusWriter handles Status().Update() and Status().Patch() calls.
//
// It exists to solve a specific problem in the reconciler's control flow.
// During FunctionComposer.Compose(), the XR's entire backing map is replaced
// with just the desired state from the function pipeline (via
// xfn.FromStruct). This wipes the XR's spec. The composer then calls
// Status().Patch() to persist the desired status. In production, the API
// server responds with the full object (spec + merged status), and
// controller-runtime writes this response back into the XR variable. The
// reconciler then continues to read spec fields (e.g. resourceRefs) and set
// conditions on the XR.
//
// This writer mimics that behavior: it takes the stored version of the
// resource (which still has the full spec from when it was loaded) and
// replaces only its top-level "status" key with the incoming object's status.
// It then writes the merged result back into the caller's object. This is not
// a general-purpose SSA merge -- it's a single key replacement that is
// sufficient because the reconciler only has one status writer producing one
// status blob.
type inMemoryStatusWriter struct {
	client *InMemoryClient
}

// Update replaces the stored resource's status with the incoming object's
// status, then writes the full merged resource (original spec + new status)
// back into obj.
func (w *inMemoryStatusWriter) Update(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)

	merged := w.mergeStatus(key, u)
	w.client.store[key] = *merged
	w.client.updated = append(w.client.updated, *merged.DeepCopy())

	// Write the merged state back into the caller's object so the reconciler
	// sees the full resource (spec + status) after this call.
	copyInto(obj, merged)

	return nil
}

// Patch replaces the stored resource's status with the incoming object's
// status, then writes the full merged resource (original spec + new status)
// back into obj.
func (w *inMemoryStatusWriter) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
	u := toUnstructured(obj)
	key := keyForUnstructured(u)

	merged := w.mergeStatus(key, u)
	w.client.store[key] = *merged
	w.client.applied = append(w.client.applied, *merged.DeepCopy())

	// Write the merged state back so the reconciler sees the full object.
	copyInto(obj, merged)

	return nil
}

// Create is not used for status subresources.
func (w *inMemoryStatusWriter) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	return nil
}

// Apply is not used for status subresources.
func (w *inMemoryStatusWriter) Apply(_ context.Context, _ runtime.ApplyConfiguration, _ ...client.SubResourceApplyOption) error {
	return nil
}

// mergeStatus replaces the stored resource's top-level "status" key with the
// incoming resource's "status" key. Everything else (spec, metadata, etc.)
// comes from the stored version. This isn't a deep merge or an SSA-style
// field-ownership merge -- it's a wholesale replacement of one top-level key.
// That's sufficient here because the reconciler has a single status writer
// producing a complete status blob; there are no concurrent partial status
// updates to reconcile.
func (w *inMemoryStatusWriter) mergeStatus(key storeKey, incoming *unstructured.Unstructured) *unstructured.Unstructured {
	stored, ok := w.client.store[key]
	if !ok {
		// No stored version; use the incoming as-is.
		return incoming.DeepCopy()
	}

	merged := stored.DeepCopy()

	// Replace the stored status with the incoming status.
	if status, ok := incoming.Object["status"]; ok {
		merged.Object["status"] = status
	}

	// Legacy-schema XRs store conditions at the top level rather than under
	// status.crossplane.
	if conditions, ok := incoming.Object["conditions"]; ok {
		merged.Object["conditions"] = conditions
	}

	return merged
}

// inMemorySubResourceClient satisfies client.SubResourceClient.
type inMemorySubResourceClient struct {
	client *InMemoryClient
}

func (s *inMemorySubResourceClient) Get(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceGetOption) error {
	return nil
}

func (s *inMemorySubResourceClient) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	return nil
}

func (s *inMemorySubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return (&inMemoryStatusWriter{client: s.client}).Update(ctx, obj, opts...)
}

func (s *inMemorySubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return (&inMemoryStatusWriter{client: s.client}).Patch(ctx, obj, patch, opts...)
}

func (s *inMemorySubResourceClient) Apply(_ context.Context, _ runtime.ApplyConfiguration, _ ...client.SubResourceApplyOption) error {
	return nil
}

// toUnstructured converts a client.Object to *unstructured.Unstructured.
func toUnstructured(obj client.Object) *unstructured.Unstructured {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u
	}

	// For typed objects, convert via the runtime converter.
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		// This shouldn't happen for well-formed objects.
		return &unstructured.Unstructured{}
	}

	u := &unstructured.Unstructured{Object: data}
	u.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	return u
}

// keyForUnstructured builds a storeKey from an unstructured resource.
func keyForUnstructured(u *unstructured.Unstructured) storeKey {
	return storeKey{
		GroupVersionKind: u.GroupVersionKind(),
		NamespacedName: types.NamespacedName{
			Namespace: u.GetNamespace(),
			Name:      u.GetName(),
		},
	}
}

// copyInto copies the content of src into dst.
func copyInto(dst client.Object, src *unstructured.Unstructured) {
	if u, ok := dst.(*unstructured.Unstructured); ok {
		src.DeepCopyInto(u)
		return
	}
	// For typed objects, best-effort conversion.
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(src.Object, dst)
}

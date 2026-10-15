/*
Copyright 2026 The Crossplane Authors.

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

package composition

import (
	"context"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

// defaultRevisionHistoryLimit is used when a Composition's revision history
// limit is unset. It matches the limit's API default.
const defaultRevisionHistoryLimit int64 = 1

// A RevisionGarbageCollector deletes CompositionRevisions that are no longer
// needed. It returns the number of revisions it deleted.
type RevisionGarbageCollector interface {
	GarbageCollect(ctx context.Context, comp *v1.Composition, revs []v1.CompositionRevision) (int, error)
}

// A RevisionGarbageCollectorFn deletes CompositionRevisions that are no longer
// needed.
type RevisionGarbageCollectorFn func(ctx context.Context, comp *v1.Composition, revs []v1.CompositionRevision) (int, error)

// GarbageCollect calls fn.
func (fn RevisionGarbageCollectorFn) GarbageCollect(ctx context.Context, comp *v1.Composition, revs []v1.CompositionRevision) (int, error) {
	return fn(ctx, comp, revs)
}

// A NopRevisionGarbageCollector never deletes any revisions.
type NopRevisionGarbageCollector struct{}

// GarbageCollect does nothing.
func (NopRevisionGarbageCollector) GarbageCollect(_ context.Context, _ *v1.Composition, _ []v1.CompositionRevision) (int, error) {
	return 0, nil
}

// An APIRevisionGarbageCollector deletes CompositionRevisions that aren't the
// latest revision, aren't used by any composite resource, and are beyond the
// Composition's revision history limit.
type APIRevisionGarbageCollector struct {
	// client is used to read XRDs and delete revisions.
	client client.Client

	// xrs is used to list composite resources. It should be backed by the
	// cache the composite resource controllers use.
	xrs client.Reader
}

// NewAPIRevisionGarbageCollector returns a RevisionGarbageCollector that uses
// the supplied clients. The XR reader should be cached, because the garbage
// collector lists every composite resource of a Composition's type.
func NewAPIRevisionGarbageCollector(c client.Client, xrs client.Reader) *APIRevisionGarbageCollector {
	return &APIRevisionGarbageCollector{client: c, xrs: xrs}
}

// GarbageCollect deletes the supplied Composition's unneeded revisions. It
// deletes nothing if it can't determine which revisions are in use.
func (gc *APIRevisionGarbageCollector) GarbageCollect(ctx context.Context, comp *v1.Composition, revs []v1.CompositionRevision) (int, error) {
	limit := defaultRevisionHistoryLimit
	if comp.Spec.RevisionHistoryLimit != nil {
		limit = *comp.Spec.RevisionHistoryLimit
	}

	// A limit of 0 disables garbage collection.
	if limit <= 0 {
		return 0, nil
	}

	// The composition controller *should* only pass revisions that are owned by
	// the given composition, but filter any others out just in case.
	owned := make([]v1.CompositionRevision, 0, len(revs))
	for _, rev := range revs {
		if metav1.IsControlledBy(&rev, comp) {
			owned = append(owned, rev)
		}
	}

	// Take the fast path if there aren't enough revisions for GC to run.
	if int64(len(owned)) <= limit+1 {
		return 0, nil
	}

	// Sort oldest to newest and remove the latest revision from the list since
	// we never delete it.
	slices.SortStableFunc(owned, func(a, b v1.CompositionRevision) int {
		return int(a.Spec.Revision - b.Spec.Revision)
	})
	owned = owned[:len(owned)-1]

	inUse, err := gc.revisionsInUse(ctx, comp, owned)
	if err != nil {
		return 0, err
	}

	// We can delete up to this many revisions, but we may not be able to delete
	// this many depending which revisions are in use.
	maxDelete := len(owned) - int(limit)
	deleted := 0

	for _, rev := range owned {
		if deleted == maxDelete {
			break
		}

		if inUse[rev.GetName()] {
			continue
		}

		if err := resource.IgnoreNotFound(gc.client.Delete(ctx, &rev)); err != nil {
			return deleted, errors.Wrapf(err, "cannot delete CompositionRevision %s", rev.Name)
		}

		deleted++
	}

	return deleted, nil
}

// revisionsInUse returns the names of the supplied revisions that composite
// resources use, or might soon use. Revisions must be sorted oldest to newest.
func (gc *APIRevisionGarbageCollector) revisionsInUse(ctx context.Context, comp *v1.Composition, revs []v1.CompositionRevision) (map[string]bool, error) {
	gv, err := schema.ParseGroupVersion(comp.Spec.CompositeTypeRef.APIVersion)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse composite type reference API version")
	}

	xrs := &kunstructured.UnstructuredList{}
	xrs.SetGroupVersionKind(gv.WithKind(comp.Spec.CompositeTypeRef.Kind + "List"))

	if err := gc.xrs.List(ctx, xrs); err != nil {
		return nil, errors.Wrap(err, "cannot list composite resources")
	}

	inUse := map[string]bool{}

	// Label selectors that might select one of this Composition's revisions.
	// An XR uses the newest revision that matches its selector.
	var selectors []labels.Selector

	for i := range xrs.Items {
		// We don't know whether this XR uses the v2 or legacy schema, so check
		// both.
		for _, s := range []composite.Schema{composite.SchemaModern, composite.SchemaLegacy} {
			xr := &composite.Unstructured{Unstructured: xrs.Items[i], Schema: s}

			// Revision names are unique across all Compositions, so we don't
			// need to check which Composition this XR uses.
			if ref := xr.GetCompositionRevisionReference(); ref != nil && ref.Name != "" {
				inUse[ref.Name] = true
			}

			// An XR without a Composition reference might select this
			// Composition when it's next reconciled.
			if ref := xr.GetCompositionReference(); ref != nil && ref.Name != "" && ref.Name != comp.GetName() {
				continue
			}

			if sel := xr.GetCompositionRevisionSelector(); sel != nil {
				selectors = append(selectors, labels.SelectorFromSet(sel.MatchLabels))
			}
		}
	}

	// XRs that don't specify a revision reference or selector get the XRD's
	// default revision selector.
	xrds := &v1.CompositeResourceDefinitionList{}
	if err := gc.client.List(ctx, xrds); err != nil {
		return nil, errors.Wrap(err, "cannot list CompositeResourceDefinitions")
	}

	for _, xrd := range xrds.Items {
		if xrd.Spec.Group != gv.Group || xrd.Spec.Names.Kind != comp.Spec.CompositeTypeRef.Kind {
			continue
		}

		if sel := xrd.Spec.DefaultCompositionRevisionSelector; sel != nil {
			selectors = append(selectors, labels.SelectorFromSet(sel.MatchLabels))
		}
	}

	for _, sel := range selectors {
		for _, rev := range slices.Backward(revs) {
			if sel.Matches(labels.Set(rev.GetLabels())) {
				inUse[rev.GetName()] = true
				break
			}
		}
	}

	return inUse, nil
}

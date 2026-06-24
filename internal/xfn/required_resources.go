/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"maps"
	"sort"

	"google.golang.org/protobuf/proto"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/step"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// MaxRequirementsIterations is the maximum number of times a Function should be
// called, limiting the number of times it can request for required resources,
// capped for safety.
const MaxRequirementsIterations = 5

// An RequiredResourcesFetcher gets required resources matching a selector.
type RequiredResourcesFetcher interface {
	Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)
}

// An RequiredResourcesFetcherFn gets required resources matching the selector.
type RequiredResourcesFetcherFn func(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)

// Fetch gets required resources matching the selector.
func (fn RequiredResourcesFetcherFn) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	return fn(ctx, rs)
}

// A RequirementsRecorder remembers the requirements a function last returned for
// a composite resource (XR), so they can be pre-satisfied next reconcile. The
// FetchingFunctionRunner defaults to one that remembers nothing.
type RequirementsRecorder interface {
	// Get returns the requirements the named function last returned for the XR
	// with the supplied UID, if any.
	Get(xrUID, function string) (*fnv1.Requirements, bool)

	// Set records the requirements the named function returned for the XR with
	// the supplied UID and kind.
	Set(xrUID string, gvk schema.GroupVersionKind, function string, r *fnv1.Requirements)
}

// A FetchingFunctionRunner wraps an underlyin FunctionRunner, adding support
// for fetching any required resources requested by the function it runs.
type FetchingFunctionRunner struct {
	wrapped   FunctionRunner
	resources RequiredResourcesFetcher
	schemas   RequiredSchemasFetcher

	// recorder remembers the requirements each function returns and lets the
	// runner pre-satisfy them on the first call of the next reconcile, to avoid
	// always calling a function twice (once to discover its requirements, once
	// to confirm they're satisfied). It defaults to remembering nothing, so the
	// runner always discovers requirements afresh.
	recorder RequirementsRecorder

	// watcher starts watches for the kinds of resource a function requires, once
	// those requirements stabilize. This lets an XR controller reconcile an XR
	// when a resource its pipeline requires changes, not only when the XR or its
	// composed resources change. It defaults to a no-op.
	watcher RequiredResourceWatcher

	// tagPerIteration makes the runner recompute the request's tag on each
	// iteration. It's needed when a response cache sits below the runner, so the
	// cache keys on the resolved request. It's off by default - when the cache
	// sits above the runner, retagging would be wasted work.
	tagPerIteration bool
}

// A FetchingFunctionRunnerOption configures a FetchingFunctionRunner.
type FetchingFunctionRunnerOption func(*FetchingFunctionRunner)

// WithRequirementsRecorder configures a FetchingFunctionRunner to remember the
// requirements functions return, and pre-satisfy them on the first call of the
// next reconcile.
func WithRequirementsRecorder(rec RequirementsRecorder) FetchingFunctionRunnerOption {
	return func(r *FetchingFunctionRunner) {
		r.recorder = rec
	}
}

// WithRequiredResourceWatcher configures a FetchingFunctionRunner to start
// watches for the kinds of resource a function requires, once those requirements
// stabilize.
func WithRequiredResourceWatcher(w RequiredResourceWatcher) FetchingFunctionRunnerOption {
	return func(r *FetchingFunctionRunner) {
		r.watcher = w
	}
}

// WithPerIterationTagging makes a FetchingFunctionRunner recompute the request's
// tag on each iteration of its requirement-resolution loop. Use it when a
// response cache sits below the runner, so the cache keys on each iteration's
// resolved request - and so a change to a required resource invalidates the
// cache.
func WithPerIterationTagging() FetchingFunctionRunnerOption {
	return func(r *FetchingFunctionRunner) {
		r.tagPerIteration = true
	}
}

// NewFetchingFunctionRunner returns a FunctionRunner that supports fetching
// required resources.
func NewFetchingFunctionRunner(r FunctionRunner, rf RequiredResourcesFetcher, sf RequiredSchemasFetcher, o ...FetchingFunctionRunnerOption) *FetchingFunctionRunner {
	f := &FetchingFunctionRunner{
		wrapped:   r,
		resources: rf,
		schemas:   sf,

		// Optional dependencies default to no-ops. By default we don't remember
		// requirements across reconciles, and we don't watch required resources.
		recorder: nopRequirementsRecorder{},
		watcher:  NopRequiredResourceWatcher{},
	}
	for _, fn := range o {
		fn(f)
	}
	return f
}

// RunFunction runs a function, repeatedly fetching any required resources it asks
// for. The function may be run up to MaxRequirementsIterations times.
func (c *FetchingFunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	// Used to store the requirements returned at the previous iteration.
	var requirements *fnv1.Requirements

	// Preserve bootstrap required resources and schemas from the initial request.
	bootstrapResources := maps.Clone(req.GetRequiredResources())
	bootstrapSchemas := maps.Clone(req.GetRequiredSchemas())

	// If we remember the requirements this function returned last reconcile,
	// pre-satisfy them on the first call. We also seed requirements with what we
	// remembered, so that if the function returns the same requirements on its
	// first call we recognize they've stabilized and return without a second
	// call. If our memory is wrong - the function returns different requirements
	// - we fall through to the iterative loop below, which self-corrects.
	//
	// A function must always return its requirements, so it's safe to satisfy
	// requirements it didn't ask for this reconcile: it'll simply return its
	// real (different) requirements, and we'll re-fetch them.
	xr := &kunstructured.Unstructured{}
	if err := FromStruct(xr, req.GetObserved().GetComposite().GetResource()); err != nil {
		return nil, errors.Wrap(err, "cannot load observed composite resource")
	}
	xrUID := string(xr.GetUID())

	if xrUID != "" {
		if r, ok := c.recorder.Get(xrUID, name); ok {
			if err := c.satisfy(ctx, req, r, bootstrapResources, bootstrapSchemas); err != nil {
				return nil, err
			}
			requirements = r
		}
	}

	for i := int32(0); i <= MaxRequirementsIterations; i++ {
		// Update the iteration counter in the context for downstream components.
		iterCtx := step.ContextWithStepIteration(ctx, i)

		// Recompute the request's tag, so it reflects the resources we fetched
		// for this iteration. The tag is the response cache's key. When the cache
		// sits below us, without this an iteration that fetched different
		// required resources would still carry the previous iteration's tag, and
		// the cache would serve the wrong response. It also means a change to a
		// required resource produces a different tag, so the cache misses and
		// the function re-runs - this is how required resource changes invalidate
		// the cache.
		if c.tagPerIteration {
			retag(req)
		}

		rsp, err := c.wrapped.RunFunction(iterCtx, name, req)
		if err != nil {
			// I can't think of any useful info to wrap this error with.
			return nil, err
		}

		for _, rs := range rsp.GetResults() {
			if rs.GetSeverity() == fnv1.Severity_SEVERITY_FATAL {
				// We won't iterate if the function returned a fatal result. We
				// don't remember requirements from a fatal response - we can't
				// trust them, and the function didn't get to act on them.
				return rsp, nil
			}
		}

		newRequirements := rsp.GetRequirements()
		if proto.Equal(newRequirements, requirements) {
			// The requirements stabilized, the function is done. If we know the
			// XR's identity, remember its requirements so we can pre-satisfy
			// them next reconcile, and start watches for the kinds of resource
			// they require so we reconcile the XR when one of those resources
			// changes. We need the XR's UID to key what we remember, and its
			// kind to start watches on the right controller; without an
			// identifiable XR we can do neither.
			if xrUID != "" {
				c.recorder.Set(xrUID, xr.GroupVersionKind(), name, newRequirements)
				if err := c.watcher.WatchRequiredResources(ctx, xr.GroupVersionKind(), requiredGVKs(newRequirements)); err != nil {
					return nil, errors.Wrap(err, "cannot watch required resources")
				}
			}
			return rsp, nil
		}

		// Store the requirements for the next iteration.
		requirements = newRequirements

		if err := c.satisfy(ctx, req, newRequirements, bootstrapResources, bootstrapSchemas); err != nil {
			return nil, err
		}

		// Pass down the updated context across iterations.
		req.Context = rsp.GetContext()
	}
	// The requirements didn't stabilize after the maximum number of iterations.
	return nil, errors.Errorf("requirements didn't stabilize after the maximum number of iterations (%d)", MaxRequirementsIterations)
}

// retag sets req.Meta.Tag to a tag derived from the request's content. It
// computes the tag over the request with its Meta excluded, so the tag doesn't
// depend on itself (or on the capabilities Crossplane advertises). This matches
// how the composer computes the initial tag - it calls Tag before setting
// req.Meta - so retagging an unchanged request reproduces the composer's tag.
func retag(req *fnv1.RunFunctionRequest) {
	meta := req.GetMeta()
	req.Meta = nil
	tag := Tag(req)
	if meta == nil {
		meta = &fnv1.RequestMeta{}
	}
	meta.Tag = tag
	req.Meta = meta
}

// satisfy fetches the resources and schemas required by r and populates them on
// req, alongside the supplied bootstrap resources and schemas. It replaces any
// required resources and schemas already on req, so that requirements from a
// previous iteration don't leak into the next.
func (c *FetchingFunctionRunner) satisfy(ctx context.Context, req *fnv1.RunFunctionRequest, r *fnv1.Requirements, bootstrapResources map[string]*fnv1.Resources, bootstrapSchemas map[string]*fnv1.Schema) error {
	// Start from the bootstrap requirements, replacing anything we fetched for a
	// previous iteration.
	req.ExtraResources = make(map[string]*fnv1.Resources) //nolint:staticcheck // Supporting deprecated field for backward compatibility
	req.RequiredResources = maps.Clone(bootstrapResources)
	if req.RequiredResources == nil {
		req.RequiredResources = make(map[string]*fnv1.Resources)
	}
	req.RequiredSchemas = maps.Clone(bootstrapSchemas)
	if req.RequiredSchemas == nil {
		req.RequiredSchemas = make(map[string]*fnv1.Schema)
	}

	// Fetch the requested resources. Support both the old (extra_resources) and
	// new (resources) field names.
	for name, selector := range r.GetExtraResources() { //nolint:staticcheck // Supporting deprecated field for backward compatibility
		resources, err := c.resources.Fetch(ctx, selector)
		if err != nil {
			return errors.Wrapf(err, "fetching resources for %s", name)
		}

		// Resources would be nil in case of not found resources.
		req.ExtraResources[name] = resources //nolint:staticcheck // Supporting deprecated field for backward compatibility
	}

	for name, selector := range r.GetResources() {
		resources, err := c.resources.Fetch(ctx, selector)
		if err != nil {
			return errors.Wrapf(err, "fetching resources for %s", name)
		}

		// Resources would be nil in case of not found resources.
		req.RequiredResources[name] = resources
	}

	for name, selector := range r.GetSchemas() {
		schema, err := c.schemas.Fetch(ctx, selector)
		if err != nil {
			return errors.Wrapf(err, "fetching schema for %s", name)
		}

		req.RequiredSchemas[name] = schema
	}

	return nil
}

// ExistingRequiredResourcesFetcher fetches required resources requested by
// functions using the provided client.Reader.
type ExistingRequiredResourcesFetcher struct {
	client client.Reader
}

// NewExistingRequiredResourcesFetcher returns a new ExistingRequiredResourcesFetcher.
func NewExistingRequiredResourcesFetcher(c client.Reader) *ExistingRequiredResourcesFetcher {
	return &ExistingRequiredResourcesFetcher{client: c}
}

// Fetch fetches resources requested by functions using the provided client.Reader.
func (e *ExistingRequiredResourcesFetcher) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	if rs == nil {
		return nil, errors.New("you must specify a resource selector")
	}

	// Handle match by name — fetch a single resource.
	if _, ok := rs.GetMatch().(*fnv1.ResourceSelector_MatchName); ok {
		r := &kunstructured.Unstructured{}
		r.SetAPIVersion(rs.GetApiVersion())
		r.SetKind(rs.GetKind())
		nn := types.NamespacedName{Namespace: rs.GetNamespace(), Name: rs.GetMatchName()}

		err := e.client.Get(ctx, nn, r)
		if kerrors.IsNotFound(err) {
			// The resource doesn't exist. We'll return nil, which the Functions
			// know means that the resource was not found.
			return nil, nil
		}

		if err != nil {
			return nil, errors.Wrap(err, "cannot get required resource by name")
		}

		o, err := AsStruct(r)
		if err != nil {
			return nil, errors.Wrap(err, "cannot encode required resource to protobuf Struct")
		}

		return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: o}}}, nil
	}

	// List resources. If match labels are specified, filter by them. If no
	// match is specified, list all resources of this apiVersion and kind.
	list := &kunstructured.UnstructuredList{}
	list.SetAPIVersion(rs.GetApiVersion())
	list.SetKind(rs.GetKind())

	opts := []client.ListOption{client.InNamespace(rs.GetNamespace())}
	if match, ok := rs.GetMatch().(*fnv1.ResourceSelector_MatchLabels); ok {
		opts = append(opts, client.MatchingLabels(match.MatchLabels.GetLabels()))
	}

	if err := e.client.List(ctx, list, opts...); err != nil {
		return nil, errors.Wrap(err, "cannot list required resources")
	}

	// Sort items by namespace and name so that the order is stable across
	// calls, even when listing across all namespaces.
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].GetNamespace()+"/"+list.Items[i].GetName() < list.Items[j].GetNamespace()+"/"+list.Items[j].GetName()
	})

	resources := make([]*fnv1.Resource, len(list.Items))
	for i, r := range list.Items {
		o, err := AsStruct(&r)
		if err != nil {
			return nil, errors.Wrap(err, "cannot encode required resource to protobuf Struct")
		}
		resources[i] = &fnv1.Resource{Resource: o}
	}

	return &fnv1.Resources{Items: resources}, nil
}

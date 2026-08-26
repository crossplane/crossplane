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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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

// A FetchingFunctionRunner wraps an underlyin FunctionRunner, adding support
// for fetching any required resources requested by the function it runs.
type FetchingFunctionRunner struct {
	wrapped   FunctionRunner
	resources RequiredResourcesFetcher
	schemas   RequiredSchemasFetcher
}

// NewFetchingFunctionRunner returns a FunctionRunner that supports fetching
// required resources.
func NewFetchingFunctionRunner(r FunctionRunner, rf RequiredResourcesFetcher, sf RequiredSchemasFetcher) *FetchingFunctionRunner {
	return &FetchingFunctionRunner{wrapped: r, resources: rf, schemas: sf}
}

// RunFunction runs a function, repeatedly fetching any required resources it asks
// for. The function may be run up to MaxRequirementsIterations times.
func (c *FetchingFunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) { //nolint:gocognit // It's essentially one loop with a series of flat conditionals; it doesn't read as complex.
	// The requirements returned at the previous iteration.
	var requirements *fnv1.Requirements

	// The resources and schemas the function was given up front - e.g. a
	// Composition's bootstrap requirements, or resources seeded from a previous
	// reconcile. We preserve them across iterations.
	bootstrapResources := maps.Clone(req.GetRequiredResources())
	bootstrapSchemas := maps.Clone(req.GetRequiredSchemas())

	for i := int32(0); i <= MaxRequirementsIterations; i++ {
		// Update the iteration counter in the context for downstream components.
		iterCtx := step.ContextWithStepIteration(ctx, i)

		rsp, err := c.wrapped.RunFunction(iterCtx, name, req)
		if err != nil {
			// I can't think of any useful info to wrap this error with.
			return nil, err
		}

		for _, rs := range rsp.GetResults() {
			if rs.GetSeverity() == fnv1.Severity_SEVERITY_FATAL {
				// We won't iterate if the function returned a fatal result.
				return rsp, nil
			}
		}

		reqs := rsp.GetRequirements()
		if proto.Equal(reqs, requirements) {
			// The requirements stabilized, the function is done.
			return rsp, nil
		}

		// Resolve the resources and schemas the function requires, starting from
		// what it was given up front and fetching what it asked for. We support
		// both the current (resources) and deprecated (extra_resources) fields.
		next := required{
			resources: maps.Clone(bootstrapResources),
			extra:     make(map[string]*fnv1.Resources),
			schemas:   maps.Clone(bootstrapSchemas),
		}
		if next.resources == nil {
			next.resources = make(map[string]*fnv1.Resources)
		}
		if next.schemas == nil {
			next.schemas = make(map[string]*fnv1.Schema)
		}
		for n, selector := range reqs.GetExtraResources() { //nolint:staticcheck // Supporting deprecated field for backward compatibility.
			res, err := c.resources.Fetch(ctx, selector)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching resources for %s", n)
			}
			// res would be nil in case of not found resources.
			next.extra[n] = res
		}
		for n, selector := range reqs.GetResources() {
			res, err := c.resources.Fetch(ctx, selector)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching resources for %s", n)
			}
			// res would be nil in case of not found resources.
			next.resources[n] = res
		}
		for n, selector := range reqs.GetSchemas() {
			s, err := c.schemas.Fetch(ctx, selector)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching schema for %s", n)
			}
			next.schemas[n] = s
		}

		// If the resources and schemas we already sent the function
		// match what its current requirements resolve to, it saw all of
		// its requirements on this call, so its response is final and
		// we needn't call it again.
		//
		// This is sound because a second call would receive a request
		// identical to the one we just sent, and a function is a
		// deterministic function of its request - so it would return
		// the same requirements, which is precisely the stabilized
		// state the check above waits for. Returning here just detects
		// that one call early. Nor can it hide a requirement grown from
		// what the function read: if reading what we sent led it to ask
		// for something new, that requirement is in reqs now, won't
		// match what we sent, and we'll fall through to fetch it and
		// iterate.
		//
		// We only do this on the first call - the one request we didn't
		// build ourselves from the function's own requirements, but was
		// handed to us pre-populated (e.g. bootstrap requirements, or
		// resources seeded from a previous reconcile). Later calls rely
		// on the stabilized check above, so a function whose
		// requirements never settle still errors rather than returning
		// early here.
		if i == 0 &&
			maps.EqualFunc(req.GetRequiredResources(), next.resources, resourcesEqual) &&
			maps.EqualFunc(req.GetExtraResources(), next.extra, resourcesEqual) && //nolint:staticcheck // Supporting deprecated field for backward compatibility.
			maps.EqualFunc(req.GetRequiredSchemas(), next.schemas, schemasEqual) {
			return rsp, nil
		}

		requirements = reqs
		req.RequiredResources = next.resources
		req.ExtraResources = next.extra //nolint:staticcheck // Supporting deprecated field for backward compatibility.
		req.RequiredSchemas = next.schemas

		// Pass down the updated context across iterations.
		req.Context = rsp.GetContext()
	}
	// The requirements didn't stabilize after the maximum number of iterations.
	return nil, errors.Errorf("requirements didn't stabilize after the maximum number of iterations (%d)", MaxRequirementsIterations)
}

// required is the resources and schemas a function's requirements resolve to.
type required struct {
	resources map[string]*fnv1.Resources
	extra     map[string]*fnv1.Resources // The deprecated extra_resources field.
	schemas   map[string]*fnv1.Schema
}

func resourcesEqual(a, b *fnv1.Resources) bool { return proto.Equal(a, b) }

func schemasEqual(a, b *fnv1.Schema) bool { return proto.Equal(a, b) }

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
		// Build a single combined label selector from both equality labels
		// and set-based match expressions.
		selector := labels.NewSelector()
		if len(match.MatchLabels.GetLabels()) > 0 {
			selector = labels.SelectorFromSet(match.MatchLabels.GetLabels())
		}
		if len(match.MatchLabels.GetExpressions()) > 0 {
			exprSelector, err := MatchExpressionsToSelector(match.MatchLabels.GetExpressions())
			if err != nil {
				return nil, errors.Wrap(err, "cannot build label selector from match expressions")
			}
			reqs, _ := exprSelector.Requirements()
			selector = selector.Add(reqs...)
		}
		opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
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

// MatchExpressionsToSelector converts protobuf MatchExpressions to a
// Kubernetes labels.Selector.
func MatchExpressionsToSelector(exprs []*fnv1.MatchExpression) (labels.Selector, error) {
	selector := labels.NewSelector()
	for _, expr := range exprs {
		op, err := ToSelectionOperator(expr.GetOperator())
		if err != nil {
			return nil, errors.Wrapf(err, "invalid match expression operator for key %q", expr.GetKey())
		}
		req, err := labels.NewRequirement(expr.GetKey(), op, expr.GetValues())
		if err != nil {
			return nil, errors.Wrapf(err, "invalid match expression for key %q", expr.GetKey())
		}
		selector = selector.Add(*req)
	}
	return selector, nil
}

// ToSelectionOperator converts a string operator to a Kubernetes
// selection.Operator.
func ToSelectionOperator(op string) (selection.Operator, error) {
	switch metav1.LabelSelectorOperator(op) {
	case metav1.LabelSelectorOpIn:
		return selection.In, nil
	case metav1.LabelSelectorOpNotIn:
		return selection.NotIn, nil
	case metav1.LabelSelectorOpExists:
		return selection.Exists, nil
	case metav1.LabelSelectorOpDoesNotExist:
		return selection.DoesNotExist, nil
	default:
		return "", errors.Errorf("unsupported match expression operator %q; supported operators are: In, NotIn, Exists, DoesNotExist", op)
	}
}

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

package composite

import (
	"context"
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
)

// MaxRequirementsIterations is the maximum number of times a Function should be
// called, limiting the number of times it can request for extra resources,
// capped for safety.
const MaxRequirementsIterations = 5

// A FetchingFunctionRunner wraps an underlying FunctionRunner, adding support
// for fetching any extra resources requested by the function it runs.
type FetchingFunctionRunner struct {
	wrapped   FunctionRunner
	resources ExtraResourcesFetcher
}

// NewFetchingFunctionRunner returns a FunctionRunner that supports fetching
// extra resources.
func NewFetchingFunctionRunner(r FunctionRunner, f ExtraResourcesFetcher) *FetchingFunctionRunner {
	return &FetchingFunctionRunner{wrapped: r, resources: f}
}

// RunFunction runs a function, repeatedly fetching any extra resources it asks
// for. The function may be run up to MaxRequirementsIterations times.
func (c *FetchingFunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	// Used to store the requirements returned at the previous iteration.
	var requirements *fnv1.Requirements

	for i := int64(0); i <= MaxRequirementsIterations; i++ {
		rsp, err := c.wrapped.RunFunction(ctx, name, req)
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

		newRequirements := rsp.GetRequirements()
		if reflect.DeepEqual(newRequirements, requirements) {
			// The requirements stabilized, the function is done.
			return rsp, nil
		}

		// Store the requirements for the next iteration.
		requirements = newRequirements

		// Cleanup the extra resources from the previous iteration to store the new ones
		req.ExtraResources = make(map[string]*fnv1.Resources)

		// Fetch the requested resources and add them to the desired state.
		for name, selector := range newRequirements.GetExtraResources() {
			resources, err := c.resources.Fetch(ctx, selector)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching resources for %s", name)
			}

			// Resources would be nil in case of not found resources.
			req.ExtraResources[name] = resources
		}

		// Pass down the updated context across iterations.
		req.Context = rsp.GetContext()
	}
	// The requirements didn't stabilize after the maximum number of iterations.
	return nil, errors.Errorf("requirements didn't stabilize after the maximum number of iterations (%d)", MaxRequirementsIterations)
}

// ExistingExtraResourcesFetcher fetches extra resources requested by
// functions using the provided client.Reader.
type ExistingExtraResourcesFetcher struct {
	client client.Reader
}

// NewExistingExtraResourcesFetcher returns a new ExistingExtraResourcesFetcher.
func NewExistingExtraResourcesFetcher(c client.Reader) *ExistingExtraResourcesFetcher {
	return &ExistingExtraResourcesFetcher{client: c}
}

// Fetch fetches resources requested by functions using the provided client.Reader.
func (e *ExistingExtraResourcesFetcher) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	if rs == nil {
		return nil, errors.New(errNilResourceSelector)
	}
	switch match := rs.GetMatch().(type) {
	case *fnv1.ResourceSelector_MatchName:
		// Fetch a single resource.
		r := &kunstructured.Unstructured{}
		r.SetAPIVersion(rs.GetApiVersion())
		r.SetKind(rs.GetKind())
		nn := types.NamespacedName{Name: rs.GetMatchName()}
		err := e.client.Get(ctx, nn, r)
		if kerrors.IsNotFound(err) {
			// The resource doesn't exist. We'll return nil, which the Functions
			// know means that the resource was not found.
			return nil, nil
		}
		if err != nil {
			return nil, errors.Wrap(err, errGetExtraResourceByName)
		}
		o, err := AsStruct(r)
		if err != nil {
			return nil, errors.Wrap(err, errExtraResourceAsStruct)
		}
		return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: o}}}, nil
	case *fnv1.ResourceSelector_MatchLabels:
		// Fetch a list of resources.
		list := &kunstructured.UnstructuredList{}
		list.SetAPIVersion(rs.GetApiVersion())
		list.SetKind(rs.GetKind())

		if err := e.client.List(ctx, list, client.MatchingLabels(match.MatchLabels.GetLabels())); err != nil {
			return nil, errors.Wrap(err, errListExtraResources)
		}

		resources := make([]*fnv1.Resource, len(list.Items))
		for i, r := range list.Items {
			o, err := AsStruct(&r)
			if err != nil {
				return nil, errors.Wrap(err, errExtraResourceAsStruct)
			}
			resources[i] = &fnv1.Resource{Resource: o}
		}

		return &fnv1.Resources{Items: resources}, nil
	}
	return nil, errors.New(errUnknownResourceSelector)
}

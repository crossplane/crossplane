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
	"reflect"

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

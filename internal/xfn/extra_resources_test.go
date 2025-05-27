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
	"fmt"
	"math/rand"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/internal/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

var _ FunctionRunner = &FetchingFunctionRunner{}

func TestFetchingFunctionRunner(t *testing.T) {
	coolResource := proto.MustStruct(map[string]any{
		"apiVersion": "test.crossplane.io/v1",
		"Kind":       "CoolResource",
		"metadata": map[string]any{
			"name": "pretty-cool",
		},
	})

	// Used in the Success test
	called := false

	type params struct {
		wrapped   composite.FunctionRunner
		resources composite.ExtraResourcesFetcher
	}
	type args struct {
		ctx  context.Context
		name string
		req  *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"RunFunctionError": {
			reason: "We should return an error if the wrapped FunctionRunner does",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return nil, errors.New("boom")
				}),
			},
			args: args{},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"FatalResult": {
			reason: "We should return early if the function returns a fatal result",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Results: []*fnv1.Result{
							{
								Severity: fnv1.Severity_SEVERITY_FATAL,
							},
						},
					}
					return rsp, nil
				}),
			},
			args: args{},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
						},
					},
				},
				err: nil,
			},
		},
		"NoRequirements": {
			reason: "We should return the response unchanged if there are no requirements",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Results: []*fnv1.Result{
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
							},
						},
					}
					return rsp, nil
				}),
			},
			args: args{},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
						},
					},
				},
				err: nil,
			},
		},
		"FetchResourcesError": {
			reason: "We should return any error encountered when fetching extra resources",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Requirements: &fnv1.Requirements{
							ExtraResources: map[string]*fnv1.ResourceSelector{
								"gimme": {
									ApiVersion: "test.crossplane.io/v1",
									Kind:       "CoolResource",
								},
							},
						},
					}
					return rsp, nil
				}),
				resources: ExtraResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
					return nil, errors.New("boom")
				}),
			},
			args: args{
				req: &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"RequirementsDidntStabilizeError": {
			reason: "We should return an error if the function's requirements never stabilize",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Requirements: &fnv1.Requirements{
							ExtraResources: map[string]*fnv1.ResourceSelector{
								"gimme": {
									ApiVersion: "test.crossplane.io/v1",

									// What are the chances we get the same number 5 times in a row?
									Kind: fmt.Sprintf("CoolResource%d", rand.Int31()),
								},
							},
						},
					}
					return rsp, nil
				}),
				resources: ExtraResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
					return &fnv1.Resources{}, nil
				}),
			},
			args: args{
				req: &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We should return the fetched resources",
			params: params{
				wrapped: composite.FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					// We only expect to be sent extra resources the second time
					// we're called, in response to our requirements.
					if called {
						want := &fnv1.RunFunctionRequest{
							ExtraResources: map[string]*fnv1.Resources{
								"gimme": {
									Items: []*fnv1.Resource{{Resource: coolResource}},
								},
							},
						}

						if diff := cmp.Diff(want, req, protocmp.Transform()); diff != "" {
							t.Errorf("RunFunction(): -want, +got:\n%s", diff)
							return nil, errors.New("unexpected RunFunctionRequest")
						}
					}

					called = true

					rsp := &fnv1.RunFunctionResponse{
						Requirements: &fnv1.Requirements{
							ExtraResources: map[string]*fnv1.ResourceSelector{
								"gimme": {
									ApiVersion: "test.crossplane.io/v1",
									Kind:       "CoolResource",
								},
							},
						},
					}
					return rsp, nil
				}),
				resources: ExtraResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
					r := &fnv1.Resources{
						Items: []*fnv1.Resource{{Resource: coolResource}},
					}
					return r, nil
				}),
			},
			args: args{
				req: &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Requirements: &fnv1.Requirements{
						ExtraResources: map[string]*fnv1.ResourceSelector{
							"gimme": {
								ApiVersion: "test.crossplane.io/v1",
								Kind:       "CoolResource",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewFetchingFunctionRunner(tc.params.wrapped, tc.params.resources)
			rsp, err := r.RunFunction(tc.args.ctx, tc.args.name, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

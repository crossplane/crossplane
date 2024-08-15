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
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
)

var _ FunctionRunner = &FetchingFunctionRunner{}

func TestExistingExtraResourcesFetcherFetch(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		rs *fnv1.ResourceSelector
		c  client.Reader
	}
	type want struct {
		res *fnv1.Resources
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessMatchName": {
			reason: "We should return a valid Resources when a resource is found by name",
			args: args{
				rs: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "cool-resource",
					},
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource")
						return nil
					}),
				},
			},
			want: want{
				res: &fnv1.Resources{
					Items: []*fnv1.Resource{
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cool-resource",
								},
							}),
						},
					},
				},
			},
		},
		"SuccessMatchLabels": {
			reason: "We should return a valid Resources when a resource is found by labels",
			args: args{
				rs: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"cool": "resource",
							},
						},
					},
				},
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "Foo",
									"metadata": map[string]interface{}{
										"name": "cool-resource",
										"labels": map[string]interface{}{
											"cool": "resource",
										},
									},
								},
							},
							{
								Object: map[string]interface{}{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "Foo",
									"metadata": map[string]interface{}{
										"name": "cooler-resource",
										"labels": map[string]interface{}{
											"cool": "resource",
										},
									},
								},
							},
						}
						return nil
					}),
				},
			},
			want: want{
				res: &fnv1.Resources{
					Items: []*fnv1.Resource{
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cool-resource",
									"labels": map[string]any{
										"cool": "resource",
									},
								},
							}),
						},
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cooler-resource",
									"labels": map[string]any{
										"cool": "resource",
									},
								},
							}),
						},
					},
				},
			},
		},
		"NotFoundMatchName": {
			reason: "We should return no error when a resource is not found by name",
			args: args{
				rs: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "cool-resource",
					},
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "Foo"}, "cool-resource")),
				},
			},
			want: want{
				res: nil,
				err: nil,
			},
		},
		// NOTE(phisco): No NotFound error is returned when listing resources by labels, so there is no NotFoundMatchLabels test case.
		"ErrorMatchName": {
			reason: "We should return any other error encountered when getting a resource by name",
			args: args{
				rs: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "cool-resource",
					},
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				res: nil,
				err: errBoom,
			},
		},
		"ErrorMatchLabels": {
			reason: "We should return any other error encountered when listing resources by labels",
			args: args{
				rs: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"cool": "resource",
							},
						},
					},
				},
				c: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
			},
			want: want{
				res: nil,
				err: errBoom,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewExistingExtraResourcesFetcher(tc.args.c)
			res, err := g.Fetch(context.Background(), tc.args.rs)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.res, res, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFetchingFunctionRunner(t *testing.T) {
	coolResource := MustStruct(map[string]any{
		"apiVersion": "test.crossplane.io/v1",
		"Kind":       "CoolResource",
		"metadata": map[string]any{
			"name": "pretty-cool",
		},
	})

	// Used in the Success test
	called := false

	type params struct {
		wrapped   FunctionRunner
		resources ExtraResourcesFetcher
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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
				wrapped: FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
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

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
	"github.com/crossplane/crossplane/internal/proto"
)

var _ ExtraResourcesFetcher = &ExistingExtraResourcesFetcher{}

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
							Resource: proto.MustStruct(map[string]any{
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
							Resource: proto.MustStruct(map[string]any{
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
							Resource: proto.MustStruct(map[string]any{
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

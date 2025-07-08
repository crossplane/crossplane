/*
Copyright 2025 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1 "github.com/crossplane/crossplane/proto/fn/v1"
)

func TestTag(t *testing.T) {
	cases := map[string]struct {
		reason string
		req    *fnv1.RunFunctionRequest
		want   string
	}{
		"NilRequest": {
			reason: "It should be possible to get a tag for a request.",
			req: &fnv1.RunFunctionRequest{
				Observed: &fnv1.State{
					Composite: &fnv1.Resource{
						Resource: MustStruct(map[string]any{
							"apiVersion": "example.org/v1",
							"kind":       "Test",
						}),
					},
				},
			},
			// TODO(negz): This could change if proto wire encoding
			// changes when we update the library. If that happens
			// too often I'm fine just deleting this test.
			want: "60e065117e1992fb17c3c5ef2a50de370eeec23fbd380836d31ad388bbe4e082",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Tag(tc.req)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nTag(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAsStruct(t *testing.T) {
	type want struct {
		s   *structpb.Struct
		err error
	}

	cases := map[string]struct {
		reason string
		o      runtime.Object
		want   want
	}{
		"Unstructured": {
			reason: "It should be possible to convert a Kubernetes unstructured.Unstructured to a struct",
			o: &kunstructured.Unstructured{Object: map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "Test",
			}},
			want: want{
				s: MustStruct(map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "Test",
				}),
			},
		},
		"Composite": {
			reason: "It should be possible to convert a Crossplane composite.Unstructured to a struct",
			o: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
				Group:   "example.org",
				Version: "v1",
				Kind:    "Test",
			})),
			want: want{
				s: MustStruct(map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "Test",
				}),
			},
		},
		"ConfigMap": {
			reason: "It should be possible to convert a real, not unstructured, resource to a struct",
			o: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cool-map"},
			},
			want: want{
				s: MustStruct(map[string]any{
					"metadata": map[string]any{
						"name":              "cool-map",
						"creationTimestamp": nil,
					},
				}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, err := AsStruct(tc.o)
			if diff := cmp.Diff(tc.want.s, s, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nTag(...): -want struct, +got struct:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nTag(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFromStruct(t *testing.T) {
	type args struct {
		o runtime.Object
		s *structpb.Struct
	}
	type want struct {
		o   runtime.Object
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Unstructured": {
			reason: "It should be possible to convert a struct to a Kubernetes unstructured.Unstructured",
			args: args{
				o: &kunstructured.Unstructured{},
				s: MustStruct(map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "Test",
				}),
			},
			want: want{
				o: &kunstructured.Unstructured{Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "Test",
				}},
			},
		},
		"Composite": {
			reason: "It should be possible to convert a struct to a Crossplane composite.Unstructured",
			args: args{
				o: composite.New(),
				s: MustStruct(map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "Test",
				}),
			},
			want: want{
				o: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Test",
				})),
			},
		},
		"ConfigMap": {
			reason: "It should be possible to convert a struct to a real, not unstructured, resource",
			args: args{
				o: &corev1.ConfigMap{},
				s: MustStruct(map[string]any{
					"metadata": map[string]any{
						"name": "cool-map",
					},
				}),
			},
			want: want{
				o: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "cool-map"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := FromStruct(tc.args.o, tc.args.s)
			if diff := cmp.Diff(tc.want.o, tc.args.o, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nTag(...): -want object, +got object:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nTag(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func MustStruct(v map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(v)
	if err != nil {
		panic(err)
	}

	return s
}

/*
Copyright 2023 The Crossplane Authors.

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

package usage

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

var errBoom = errors.New("boom")

func TestResolveSelectors(t *testing.T) {
	valueTrue := true
	type args struct {
		client client.Client
		u      *v1alpha1.Usage
	}
	type want struct {
		u   *v1alpha1.Usage
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AlreadyResolved": {
			reason: "If selectors resolved already, we should do nothing.",
			args: args{
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "another",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
				},
			},
			want: want{
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "another",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
				},
			},
		},
		"AlreadyResolvedNoUsing": {
			reason: "If selectors resolved already, we should do nothing.",
			args: args{
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			want: want{
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
		},
		"CannotResolveUsedListError": {
			reason: "We should return error if we cannot list the used resources.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return errBoom
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errListResourceMatchingLabels), errResolveSelectorForUsedResource),
			},
		},
		"CannotResolveUsingListError": {
			reason: "We should return error if we cannot list the using resources.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return errBoom
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errListResourceMatchingLabels), errResolveSelectorForUsingResource),
			},
		},
		"CannotUpdateAfterResolvingUsed": {
			reason: "We should return error if we cannot update the usage after resolving used resource.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						l := list.(*composed.UnstructuredList)
						switch l.GetKind() {
						case "SomeKindList":
							l.Items = []unstructured.Unstructured{
								{
									Object: map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "SomeKind",
										"metadata": map[string]interface{}{
											"name": "some",
										},
									},
								},
							}
						default:
							t.Errorf("unexpected list kind: %s", l.GetKind())
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return errBoom
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateAfterResolveSelector),
			},
		},
		"CannotUpdateAfterResolvingUsing": {
			reason: "We should return error if we cannot update the usage after resolving using resource.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						l := list.(*composed.UnstructuredList)
						switch l.GetKind() {
						case "AnotherKindList":
							l.Items = []unstructured.Unstructured{
								{
									Object: map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "AnotherKind",
										"metadata": map[string]interface{}{
											"name": "another",
										},
									},
								},
							}
						default:
							t.Errorf("unexpected list kind: %s", l.GetKind())
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return errBoom
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateAfterResolveSelector),
			},
		},
		"CannotResolveNoMatchingResources": {
			reason: "We should return error if there are no matching resources.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return nil
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtResourcesNotFound, "SomeKind", map[string]string{"foo": "bar"}), errResolveSelectorForUsedResource),
			},
		},

		"CannotResolveNoMatchingResourcesWithControllerRef": {
			reason: "We should return error if there are no matching resources with controller ref.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						l := list.(*composed.UnstructuredList)
						switch l.GetKind() {
						case "SomeKindList":
							l.Items = []unstructured.Unstructured{
								{
									Object: map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "SomeKind",
										"metadata": map[string]interface{}{
											"name": "some",
										},
									},
								},
							}
						default:
							t.Errorf("unexpected list kind: %s", l.GetKind())
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				u: &v1alpha1.Usage{
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
								MatchControllerRef: &valueTrue,
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtResourcesNotFoundWithControllerRef, "SomeKind", map[string]string{"foo": "bar"}), errResolveSelectorForUsedResource),
			},
		},
		"BothSelectorsResolved": {
			reason: "If selectors defined for both \"of\" and \"by\", both should be resolved.",
			args: args{
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						l := list.(*composed.UnstructuredList)
						if v := l.GroupVersionKind().Version; v != "v1" {
							t.Errorf("unexpected list version: %s", v)
						}
						switch l.GetKind() {
						case "SomeKindList":
							if len(opts) != 1 && opts[0].(client.MatchingLabels)["foo"] != "bar" {
								t.Errorf("unexpected list options: %v", opts)
							}
							l.Items = []unstructured.Unstructured{
								{
									Object: map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "SomeKind",
										"metadata": map[string]interface{}{
											"name": "some",
											"ownerReferences": []interface{}{
												map[string]interface{}{
													"apiVersion": "v1",
													"kind":       "OwnerKind",
													"name":       "owner",
													"controller": true,
													"uid":        "some-uid",
												},
											},
										},
									},
								},
							}
						case "AnotherKindList":
							if len(opts) != 1 && opts[0].(client.MatchingLabels)["baz"] != "qux" {
								t.Errorf("unexpected list options: %v", opts)
							}
							l.Items = []unstructured.Unstructured{
								{
									Object: map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "AnotherKind",
										"metadata": map[string]interface{}{
											"name": "another",
										},
									},
								},
							}
						default:
							t.Errorf("unexpected list kind: %s", l.GetKind())
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				u: &v1alpha1.Usage{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "OwnerKind",
								Name:       "owner",
								Controller: &valueTrue,
								UID:        "some-uid",
							},
						},
					},
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
								MatchControllerRef: &valueTrue,
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
					Status: v1alpha1.UsageStatus{},
				},
			},
			want: want{
				u: &v1alpha1.Usage{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "OwnerKind",
								Name:       "owner",
								Controller: &valueTrue,
								UID:        "some-uid",
							},
						},
					},
					Spec: v1alpha1.UsageSpec{
						Of: v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "SomeKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "some",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
								MatchControllerRef: &valueTrue,
							},
						},
						By: &v1alpha1.Resource{
							APIVersion: "v1",
							Kind:       "AnotherKind",
							ResourceRef: &v1alpha1.ResourceRef{
								Name: "another",
							},
							ResourceSelector: &v1alpha1.ResourceSelector{
								MatchLabels: map[string]string{
									"baz": "qux",
								},
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := newAPISelectorResolver(tc.args.client)
			err := r.resolveSelectors(context.Background(), tc.args.u)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nr.resolveSelectors(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.want.u, tc.args.u); diff != "" {
				t.Errorf("%s\nr.resolveSelectors(...): -want usage, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

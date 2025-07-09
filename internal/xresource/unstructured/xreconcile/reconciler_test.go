/*
Copyright 2025 The Crossplane Authors.

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

package xreconcile

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
)

func Test_statusUnchanged(t *testing.T) {
	type args struct {
		a client.Object
		b client.Object
	}
	type want struct {
		result bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"IdenticalStatus": {
			reason: "Should return true when status is identical",
			args: args{
				a: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
					claim.WithConditions(xpv1.Condition{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
						Reason: "Available",
					}),
				),
				b: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
					claim.WithConditions(xpv1.Condition{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
						Reason: "Available",
					}),
				),
			},
			want: want{
				result: true,
			},
		},
		"DifferentStatusConditions": {
			reason: "Should return false when status conditions differ",
			args: args{
				a: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
					claim.WithConditions(xpv1.Condition{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
						Reason: "Available",
					}),
				),
				b: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
					claim.WithConditions(xpv1.Condition{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionFalse,
						Reason: "Unavailable",
					}),
				),
			},
			want: want{
				result: false,
			},
		},
		"EmptyStatus": {
			reason: "Should return true when both have empty status",
			args: args{
				a: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
				),
				b: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
				),
			},
			want: want{
				result: true,
			},
		},
		"OneEmptyOneWithStatus": {
			reason: "Should return false when one has empty status and other has conditions",
			args: args{
				a: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
				),
				b: claim.New(
					claim.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "TestClaim",
					}),
					claim.WithConditions(xpv1.Condition{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
						Reason: "Available",
					}),
				),
			},
			want: want{
				result: false,
			},
		},
		"UnstructuredIdenticalStatus": {
			reason: "Should return true for unstructured objects with identical status",
			args: args{
				a: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"status": map[string]interface{}{
							"phase": "Ready",
							"conditions": []interface{}{
								map[string]interface{}{
									"type":   "Ready",
									"status": "True",
									"reason": "Available",
								},
							},
						},
					},
				},
				b: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"status": map[string]interface{}{
							"phase": "Ready",
							"conditions": []interface{}{
								map[string]interface{}{
									"type":   "Ready",
									"status": "True",
									"reason": "Available",
								},
							},
						},
					},
				},
			},
			want: want{
				result: true,
			},
		},
		"UnstructuredDifferentStatus": {
			reason: "Should return false for unstructured objects with different status",
			args: args{
				a: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"status": map[string]interface{}{
							"phase": "Ready",
						},
					},
				},
				b: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"status": map[string]interface{}{
							"phase": "Failed",
						},
					},
				},
			},
			want: want{
				result: false,
			},
		},
		"UnstructuredNoStatus": {
			reason: "Should return true for unstructured objects with no status field",
			args: args{
				a: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
					},
				},
				b: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			want: want{
				result: true,
			},
		},
		"SpecChangedStatusSame": {
			reason: "Should return true when only spec changes but status is same",
			args: args{
				a: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"spec": map[string]interface{}{
							"field": "value1",
						},
						"status": map[string]interface{}{
							"phase": "Ready",
						},
					},
				},
				b: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "TestResource",
						"metadata": map[string]interface{}{
							"name": "test",
						},
						"spec": map[string]interface{}{
							"field": "value2",
						},
						"status": map[string]interface{}{
							"phase": "Ready",
						},
					},
				},
			},
			want: want{
				result: true,
			},
		},
		"MetadataChangedStatusSame": {
			reason: "Should return true when metadata changes but status is same",
			args: args{
				a: func() *claim.Unstructured {
					c := claim.New(
						claim.WithGroupVersionKind(schema.GroupVersionKind{
							Group:   "example.org",
							Version: "v1",
							Kind:    "TestClaim",
						}),
						claim.WithConditions(xpv1.Condition{
							Type:   xpv1.TypeReady,
							Status: corev1.ConditionTrue,
							Reason: "Available",
						}),
					)
					c.SetName("test")
					c.SetAnnotations(map[string]string{"key": "value1"})
					return c
				}(),
				b: func() *claim.Unstructured {
					c := claim.New(
						claim.WithGroupVersionKind(schema.GroupVersionKind{
							Group:   "example.org",
							Version: "v1",
							Kind:    "TestClaim",
						}),
						claim.WithConditions(xpv1.Condition{
							Type:   xpv1.TypeReady,
							Status: corev1.ConditionTrue,
							Reason: "Available",
						}),
					)
					c.SetName("test")
					c.SetAnnotations(map[string]string{"key": "value2"})
					return c
				}(),
			},
			want: want{
				result: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := statusUnchanged(logging.NewNopLogger(), tc.args.a, tc.args.b)

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nstatusUnchanged(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

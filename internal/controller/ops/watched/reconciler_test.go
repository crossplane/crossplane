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

package watched

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
)

var errBoom = errors.New("boom")

func TestReconcile(t *testing.T) {
	type params struct {
		client  client.Client
		options []ReconcilerOption
		wo      *v1alpha1.WatchOperation
	}
	type args struct {
		ctx context.Context
		req reconcile.Request
	}
	type want struct {
		result reconcile.Result
		err    error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"CreateOperationForDeletedResource": {
			reason: "Should create Operation with synthetic resource when watched resource is deleted",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if _, ok := obj.(*unstructured.Unstructured); ok {
							// Return not found for the watched resource (deletion scenario)
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							wo.Spec.OperationTemplate = v1alpha1.OperationTemplate{
								Spec: v1alpha1.OperationSpec{
									Mode: v1alpha1.OperationModePipeline,
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "test-step",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "test-function",
											},
										},
									},
								},
							}
							return nil
						}
						return errBoom
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							// Return empty list
							ol.Items = []v1alpha1.Operation{}
							return nil
						}
						return errBoom
					},
					MockCreate: test.NewMockCreateFn(nil),
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						OperationTemplate: v1alpha1.OperationTemplate{
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"GetError": {
			reason: "Should return an error if getting watched resource fails",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"WatchOperationNotFound": {
			reason: "Should return early if WatchOperation is not found",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if _, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u := obj.(*unstructured.Unstructured)
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						// Return not found for WatchOperation
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"Paused": {
			reason: "Should return early if WatchOperation is paused",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return paused WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							wo.SetAnnotations(map[string]string{
								"crossplane.io/paused": "true",
							})
							return nil
						}
						return errBoom
					},
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
						Annotations: map[string]string{
							"crossplane.io/paused": "true",
						},
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    nil,
			},
		},
		"Deleted": {
			reason: "Should return early if WatchOperation is being deleted",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return deleted WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							now := metav1.Now()
							wo.SetDeletionTimestamp(&now)
							return nil
						}
						return errBoom
					},
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    nil,
			},
		},
		"ListOperationsError": {
			reason: "Should return an error if listing Operations fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							return nil
						}
						return errBoom
					},
					MockList: test.NewMockListFn(errBoom),
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"CreateOperation": {
			reason: "Should create an Operation when watched resource changes",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							wo.Spec.OperationTemplate = v1alpha1.OperationTemplate{
								Spec: v1alpha1.OperationSpec{
									Mode: v1alpha1.OperationModePipeline,
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "test-step",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "test-function",
											},
										},
									},
								},
							}
							return nil
						}
						return errBoom
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							// Return empty list
							ol.Items = []v1alpha1.Operation{}
							return nil
						}
						return errBoom
					},
					MockCreate: test.NewMockCreateFn(nil),
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						OperationTemplate: v1alpha1.OperationTemplate{
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"OperationAlreadyExists": {
			reason: "Should not create duplicate Operation for same resource version",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							// Return watched resource
							u.SetUID("test-uid")
							u.SetResourceVersion("123")
							return nil
						}
						if wo, ok := obj.(*v1alpha1.WatchOperation); ok {
							// Return WatchOperation
							wo.SetName("test-watch")
							wo.SetUID("test-uid")
							return nil
						}
						return errBoom
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							// Return existing operation with same name
							watched := &unstructured.Unstructured{
								Object: map[string]any{
									"metadata": map[string]any{
										"uid":             "test-uid",
										"resourceVersion": "123",
									},
								},
							}
							watched.SetGroupVersionKind(schema.GroupVersionKind{
								Group:   "",
								Version: "v1",
								Kind:    "Pod",
							})
							expectedName := OperationName(&v1alpha1.WatchOperation{
								ObjectMeta: metav1.ObjectMeta{Name: "test-watch"},
							}, watched)
							ol.Items = []v1alpha1.Operation{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: expectedName,
									},
								},
							}
							return nil
						}
						return errBoom
					},
				},
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						Watch: v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-pod",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.client, tc.params.wo, tc.params.options...)
			got, err := r.Reconcile(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want result, +got result:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestOperationName(t *testing.T) {
	type args struct {
		wo      *v1alpha1.WatchOperation
		watched *unstructured.Unstructured
	}
	type want struct {
		name string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should create deterministic name from WatchOperation name and watched resource UID/version",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":            "test-pod",
							"namespace":       "default",
							"uid":             "test-uid",
							"resourceVersion": "123",
						},
					},
				},
			},
			want: want{
				name: "test-watch-2ae9bd3",
			},
		},
		"DifferentResourceVersion": {
			reason: "Should create different name for different resource version",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":            "test-pod",
							"namespace":       "default",
							"uid":             "test-uid",
							"resourceVersion": "124",
						},
					},
				},
			},
			want: want{
				name: "test-watch-a1051fc",
			},
		},
		"SyntheticDeletedResource": {
			reason: "Should create unique names for synthetic deleted resources with deletion timestamp",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":              "test-pod",
							"namespace":         "default",
							"resourceVersion":   v1alpha1.SyntheticResourceVersionDeleted,
							"deletionTimestamp": "2023-01-01T00:00:00Z", // Simulates the deletion timestamp
						},
					},
				},
			},
			want: want{
				name: "test-watch-a95edb5", // Hash includes deletion timestamp for uniqueness
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := OperationName(tc.args.wo, tc.args.watched)
			if diff := cmp.Diff(tc.want.name, got); diff != "" {
				t.Errorf("\n%s\nOperationName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewOperation(t *testing.T) {
	type args struct {
		wo      *v1alpha1.WatchOperation
		watched *unstructured.Unstructured
		name    string
	}
	type want struct {
		op *v1alpha1.Operation
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"WatchedResourceInjected": {
			reason: "Should inject watched resource into all pipeline steps",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						OperationTemplate: v1alpha1.OperationTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"template": "label",
								},
							},
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":      "test-pod",
							"namespace": "default",
						},
					},
				},
				name: "test-watch-abcdef1",
			},
			want: want{
				op: &v1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch-abcdef1",
						Labels: map[string]string{
							"template":                       "label",
							v1alpha1.LabelWatchOperationName: "test-watch",
						},
						Annotations: map[string]string{
							v1alpha1.AnnotationWatchedResourceAPIVersion:      "v1",
							v1alpha1.AnnotationWatchedResourceKind:            "Pod",
							v1alpha1.AnnotationWatchedResourceName:            "test-pod",
							v1alpha1.AnnotationWatchedResourceNamespace:       "default",
							v1alpha1.AnnotationWatchedResourceResourceVersion: "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "ops.crossplane.io/v1alpha1",
								Kind:               "WatchOperation",
								Name:               "test-watch",
								UID:                types.UID("test-uid"),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: v1alpha1.FunctionReference{
									Name: "test-function",
								},
								Requirements: &v1alpha1.FunctionRequirements{
									RequiredResources: []v1alpha1.RequiredResourceSelector{
										{
											RequirementName: v1alpha1.RequirementNameWatchedResource,
											APIVersion:      "v1",
											Kind:            "Pod",
											Namespace:       ptr.To("default"),
											Name:            ptr.To("test-pod"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"ClusterScopedResource": {
			reason: "Should inject cluster-scoped watched resource without namespace",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						OperationTemplate: v1alpha1.OperationTemplate{
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Node",
						"metadata": map[string]any{
							"name": "test-node",
						},
					},
				},
				name: "test-watch-abcdef1",
			},
			want: want{
				op: &v1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch-abcdef1",
						Labels: map[string]string{
							v1alpha1.LabelWatchOperationName: "test-watch",
						},
						Annotations: map[string]string{
							v1alpha1.AnnotationWatchedResourceAPIVersion:      "v1",
							v1alpha1.AnnotationWatchedResourceKind:            "Node",
							v1alpha1.AnnotationWatchedResourceName:            "test-node",
							v1alpha1.AnnotationWatchedResourceResourceVersion: "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "ops.crossplane.io/v1alpha1",
								Kind:               "WatchOperation",
								Name:               "test-watch",
								UID:                types.UID("test-uid"),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: v1alpha1.FunctionReference{
									Name: "test-function",
								},
								Requirements: &v1alpha1.FunctionRequirements{
									RequiredResources: []v1alpha1.RequiredResourceSelector{
										{
											RequirementName: v1alpha1.RequirementNameWatchedResource,
											APIVersion:      "v1",
											Kind:            "Node",
											Name:            ptr.To("test-node"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"PreservesExistingRequirements": {
			reason: "Should preserve existing requirements while adding watched resource",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						OperationTemplate: v1alpha1.OperationTemplate{
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
										Requirements: &v1alpha1.FunctionRequirements{
											RequiredResources: []v1alpha1.RequiredResourceSelector{
												{
													RequirementName: "existing-requirement",
													APIVersion:      "v1",
													Kind:            "Secret",
													Name:            ptr.To("existing-secret"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":      "test-pod",
							"namespace": "default",
						},
					},
				},
				name: "test-watch-abcdef1",
			},
			want: want{
				op: &v1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch-abcdef1",
						Labels: map[string]string{
							v1alpha1.LabelWatchOperationName: "test-watch",
						},
						Annotations: map[string]string{
							v1alpha1.AnnotationWatchedResourceAPIVersion:      "v1",
							v1alpha1.AnnotationWatchedResourceKind:            "Pod",
							v1alpha1.AnnotationWatchedResourceName:            "test-pod",
							v1alpha1.AnnotationWatchedResourceNamespace:       "default",
							v1alpha1.AnnotationWatchedResourceResourceVersion: "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "ops.crossplane.io/v1alpha1",
								Kind:               "WatchOperation",
								Name:               "test-watch",
								UID:                types.UID("test-uid"),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: v1alpha1.FunctionReference{
									Name: "test-function",
								},
								Requirements: &v1alpha1.FunctionRequirements{
									RequiredResources: []v1alpha1.RequiredResourceSelector{
										{
											RequirementName: "existing-requirement",
											APIVersion:      "v1",
											Kind:            "Secret",
											Name:            ptr.To("existing-secret"),
										},
										{
											RequirementName: v1alpha1.RequirementNameWatchedResource,
											APIVersion:      "v1",
											Kind:            "Pod",
											Namespace:       ptr.To("default"),
											Name:            ptr.To("test-pod"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"SyntheticDeletedResource": {
			reason: "Should create Operation with synthetic resource version for deleted resource",
			args: args{
				wo: &v1alpha1.WatchOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.WatchOperationSpec{
						OperationTemplate: v1alpha1.OperationTemplate{
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]any{
							"name":              "test-pod",
							"namespace":         "default",
							"resourceVersion":   v1alpha1.SyntheticResourceVersionDeleted,
							"deletionTimestamp": "2023-01-01T00:00:00Z", // Simulates the deletion timestamp
						},
					},
				},
				name: "test-watch-abcdef1",
			},
			want: want{
				op: &v1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-watch-abcdef1",
						Labels: map[string]string{
							v1alpha1.LabelWatchOperationName: "test-watch",
						},
						Annotations: map[string]string{
							v1alpha1.AnnotationWatchedResourceAPIVersion:      "v1",
							v1alpha1.AnnotationWatchedResourceKind:            "Pod",
							v1alpha1.AnnotationWatchedResourceName:            "test-pod",
							v1alpha1.AnnotationWatchedResourceNamespace:       "default",
							v1alpha1.AnnotationWatchedResourceResourceVersion: v1alpha1.SyntheticResourceVersionDeleted,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "ops.crossplane.io/v1alpha1",
								Kind:               "WatchOperation",
								Name:               "test-watch",
								UID:                types.UID("test-uid"),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: v1alpha1.FunctionReference{
									Name: "test-function",
								},
								Requirements: &v1alpha1.FunctionRequirements{
									RequiredResources: []v1alpha1.RequiredResourceSelector{
										{
											RequirementName: v1alpha1.RequirementNameWatchedResource,
											APIVersion:      "v1",
											Kind:            "Pod",
											Namespace:       ptr.To("default"),
											Name:            ptr.To("test-pod"),
										},
									},
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
			got := NewOperation(tc.args.wo, tc.args.watched, tc.args.name)
			if diff := cmp.Diff(tc.want.op, got); diff != "" {
				t.Errorf("\n%s\nNewOperation(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

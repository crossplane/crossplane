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
	"maps"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

var errBoom = errors.New("boom")

type testRecorder struct {
	events []event.Event
}

func (r *testRecorder) Event(_ runtime.Object, e event.Event) {
	r.events = append(r.events, e)
}

func (r *testRecorder) WithAnnotations(_ ...string) event.Recorder {
	return r
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

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
							}, watched, "")
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
			r, err := NewReconciler(tc.params.client, tc.params.wo, tc.params.options...)
			if err != nil {
				t.Fatalf("\n%s\nNewReconciler(...): %v", tc.reason, err)
			}
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

func TestReconcileWatchConditions(t *testing.T) {
	t.Parallel()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "test-cm",
		},
	}

	baseWO := func(watch v1alpha1.WatchSpec) *v1alpha1.WatchOperation {
		return &v1alpha1.WatchOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-watch",
				UID:  types.UID("test-uid"),
			},
			Spec: v1alpha1.WatchOperationSpec{
				Watch: watch,
				OperationTemplate: v1alpha1.OperationTemplate{
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{{
							Step:        "test-step",
							FunctionRef: v1alpha1.FunctionReference{Name: "test-function"},
						}},
					},
				},
			},
		}
	}

	newClient := func(t *testing.T, wo *v1alpha1.WatchOperation, rv *string, data *map[string]any, creates *int) client.Client {
		t.Helper()

		return &test.MockClient{
			MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if u, ok := obj.(*unstructured.Unstructured); ok {
					if u.GetResourceVersion() == v1alpha1.SyntheticResourceVersionDeleted {
						return nil
					}
					d := map[string]any{}
					if data != nil {
						maps.Copy(d, *data)
					}
					u.Object = map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":            "test-cm",
							"namespace":       "default",
							"uid":             "test-uid",
							"resourceVersion": *rv,
						},
						"data": d,
					}
					u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
					return nil
				}
				if got, ok := obj.(*v1alpha1.WatchOperation); ok {
					*got = *wo
					return nil
				}
				return errBoom
			},
			MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				if ol, ok := list.(*v1alpha1.OperationList); ok {
					ol.Items = nil
					return nil
				}
				return errBoom
			},
			MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				*creates++
				return nil
			},
		}
	}

	t.Run("SkipUnchangedOnChange", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
		})
		rv := "123"
		data := map[string]any{"watched": "value"}
		creates := 0

		r, err := NewReconciler(newClient(t, wo, &rv, &data, &creates), wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("first Reconcile(): %v", err)
		}
		rv = "456"
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("second Reconcile(): %v", err)
		}
		if creates != 1 {
			t.Fatalf("creates: got %d, want 1", creates)
		}
	})

	t.Run("CreateOnOnChangeValueChange", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
		})
		rv := "123"
		data := map[string]any{"watched": "one"}
		creates := 0

		r, err := NewReconciler(newClient(t, wo, &rv, &data, &creates), wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("first Reconcile(): %v", err)
		}
		data["watched"] = "two"
		rv = "456"
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("second Reconcile(): %v", err)
		}
		if creates != 2 {
			t.Fatalf("creates: got %d, want 2", creates)
		}
	})

	t.Run("SkipWhenGateFails", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
			When: []v1alpha1.WatchCondition{{
				Name:       "enabled",
				Expression: "has(object.data) && object.data['enabled'] == 'true'",
			}},
		})
		rv := "123"
		data := map[string]any{"watched": "value", "enabled": "false"}
		creates := 0

		r, err := NewReconciler(newClient(t, wo, &rv, &data, &creates), wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile(): %v", err)
		}
		if creates != 0 {
			t.Fatalf("creates: got %d, want 0", creates)
		}
	})

	t.Run("SkipDeleteWhenWhenRequiresNotDeleted", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
			When: []v1alpha1.WatchCondition{{
				Name:       "not-deleted",
				Expression: "!deleted",
			}},
		})
		creates := 0

		c := &test.MockClient{
			MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if _, ok := obj.(*unstructured.Unstructured); ok {
					return kerrors.NewNotFound(schema.GroupResource{}, "")
				}
				if got, ok := obj.(*v1alpha1.WatchOperation); ok {
					*got = *wo
					return nil
				}
				return errBoom
			},
			MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				creates++
				return nil
			},
		}

		r, err := NewReconciler(c, wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile(): %v", err)
		}
		if creates != 0 {
			t.Fatalf("creates: got %d, want 0", creates)
		}
	})

	t.Run("CreateOnDeleteWithOnChangeDeleted", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "deleted"},
		})
		creates := 0

		c := &test.MockClient{
			MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if _, ok := obj.(*unstructured.Unstructured); ok {
					return kerrors.NewNotFound(schema.GroupResource{}, "")
				}
				if got, ok := obj.(*v1alpha1.WatchOperation); ok {
					*got = *wo
					return nil
				}
				return errBoom
			},
			MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				if ol, ok := list.(*v1alpha1.OperationList); ok {
					ol.Items = nil
					return nil
				}
				return errBoom
			},
			MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				creates++
				return nil
			},
		}

		r, err := NewReconciler(c, wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile(): %v", err)
		}
		if creates != 1 {
			t.Fatalf("creates: got %d, want 1", creates)
		}
		if _, ok := r.fingerprints.Load(req.NamespacedName); ok {
			t.Fatal("expected deletion not to store a fingerprint")
		}
	})

	t.Run("SkipDeletedResourceWhenEvaluationErrors", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.missing"},
		})
		rec := &testRecorder{}

		c := &test.MockClient{
			MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if _, ok := obj.(*unstructured.Unstructured); ok {
					return kerrors.NewNotFound(schema.GroupResource{}, "")
				}
				if got, ok := obj.(*v1alpha1.WatchOperation); ok {
					*got = *wo
					return nil
				}
				return errBoom
			},
		}

		r, err := NewReconciler(c, wo, WithRecorder(rec))
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}
		r.fingerprints.Store(req.NamespacedName, `"stale"`)

		result, err := r.Reconcile(context.Background(), req)
		if err != nil {
			t.Fatalf("Reconcile(): %v", err)
		}
		if result.Requeue {
			t.Fatal("expected deleted resource evaluation failure not to requeue")
		}
		if _, ok := r.fingerprints.Load(req.NamespacedName); ok {
			t.Fatal("expected fingerprint to be deleted after failed deletion evaluation")
		}
		if len(rec.events) != 1 {
			t.Fatalf("events: got %d, want 1", len(rec.events))
		}
		if rec.events[0].Reason != reasonEvaluateWatchConditions {
			t.Fatalf("event reason: got %q, want %q", rec.events[0].Reason, reasonEvaluateWatchConditions)
		}
		if rec.events[0].Type != event.TypeWarning {
			t.Fatalf("event type: got %q, want %q", rec.events[0].Type, event.TypeWarning)
		}
		if !containsAll(rec.events[0].Message,
			"cannot evaluate watch conditions for deleted watched resource",
			"no Operation will be created",
			"Guard non-metadata field access with deleted",
		) {
			t.Fatalf("event message: got %q", rec.events[0].Message)
		}
	})

	t.Run("StoreFingerprintWhenOperationAlreadyExists", func(t *testing.T) {
		t.Parallel()

		wo := baseWO(v1alpha1.WatchSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
		})
		watched := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":            "test-cm",
				"namespace":       "default",
				"uid":             "test-uid",
				"resourceVersion": "123",
			},
			"data": map[string]any{"watched": "one"},
		}}
		watched.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
		existing := OperationName(wo, watched, `"one"`)
		creates := 0

		c := &test.MockClient{
			MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if u, ok := obj.(*unstructured.Unstructured); ok {
					*u = *watched.DeepCopy()
					return nil
				}
				if got, ok := obj.(*v1alpha1.WatchOperation); ok {
					*got = *wo
					return nil
				}
				return errBoom
			},
			MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				if ol, ok := list.(*v1alpha1.OperationList); ok {
					ol.Items = []v1alpha1.Operation{{
						ObjectMeta: metav1.ObjectMeta{Name: existing},
					}}
					return nil
				}
				return errBoom
			},
			MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				creates++
				return nil
			},
		}

		r, err := NewReconciler(c, wo)
		if err != nil {
			t.Fatalf("NewReconciler(): %v", err)
		}

		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile(): %v", err)
		}
		if creates != 0 {
			t.Fatalf("creates: got %d, want 0", creates)
		}
		got, ok := r.fingerprints.Load(req.NamespacedName)
		if !ok {
			t.Fatal("expected fingerprint to be stored for the existing operation")
		}
		if got != `"one"` {
			t.Fatalf("fingerprint: got %q, want %q", got, `"one"`)
		}
	})
}

func TestNewReconcilerInvalidWatchConditions(t *testing.T) {
	t.Parallel()

	_, err := NewReconciler(&test.MockClient{}, &v1alpha1.WatchOperation{
		Spec: v1alpha1.WatchOperationSpec{
			Watch: v1alpha1.WatchSpec{
				OnChange: &v1alpha1.WatchOnChange{Expression: "object..invalid"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestUpdateProgram(t *testing.T) {
	type args struct {
		initial v1alpha1.WatchSpec
		update  v1alpha1.WatchSpec
	}
	type want struct {
		err         error
		hasOnChange bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddsOnChange": {
			reason: "Should replace the compiled program when watch conditions change",
			args: args{
				initial: v1alpha1.WatchSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				update: v1alpha1.WatchSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
				},
			},
			want: want{
				hasOnChange: true,
			},
		},
		"InvalidExpressionKeepsPriorProgram": {
			reason: "Should keep the existing program when the updated watch conditions do not compile",
			args: args{
				initial: v1alpha1.WatchSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					OnChange:   &v1alpha1.WatchOnChange{Expression: "object.data['watched']"},
				},
				update: v1alpha1.WatchSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					OnChange:   &v1alpha1.WatchOnChange{Expression: "object..invalid"},
				},
			},
			want: want{
				err:         cmpopts.AnyError,
				hasOnChange: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := NewReconciler(&test.MockClient{}, &v1alpha1.WatchOperation{
				ObjectMeta: metav1.ObjectMeta{Name: "test-watch"},
				Spec:       v1alpha1.WatchOperationSpec{Watch: tc.args.initial},
			})
			if err != nil {
				t.Fatalf("\n%s\nNewReconciler(...): %v", tc.reason, err)
			}

			err = r.UpdateProgram(tc.args.update)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateProgram(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.hasOnChange, r.program.HasOnChange()); diff != "" {
				t.Errorf("\n%s\nUpdateProgram(...): -want HasOnChange, +got HasOnChange:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestOperationName(t *testing.T) {
	type args struct {
		wo                *v1alpha1.WatchOperation
		watched           *unstructured.Unstructured
		changeFingerprint string
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
				name: "test-watch-ef68891", // Hash includes deletion timestamp for uniqueness
			},
		},
		"OnChangeFingerprintSameVersion": {
			reason: "Should include the change fingerprint and resource version so retries of the same version stay idempotent",
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
				changeFingerprint: `"two"`,
			},
			want: want{
				name: "test-watch-a429eed",
			},
		},
		"OnChangeFingerprintDifferentVersion": {
			reason: "Should create a different name when the resource version changes and the change fingerprint stays the same",
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
							"resourceVersion": "999",
						},
					},
				},
				changeFingerprint: `"two"`,
			},
			want: want{
				name: "test-watch-5b07a81",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := OperationName(tc.args.wo, tc.args.watched, tc.args.changeFingerprint)
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
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
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
											Namespace:       new("default"),
											Name:            new("test-pod"),
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
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
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
											Name:            new("test-node"),
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
													Name:            new("existing-secret"),
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
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
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
											Name:            new("existing-secret"),
										},
										{
											RequirementName: v1alpha1.RequirementNameWatchedResource,
											APIVersion:      "v1",
											Kind:            "Pod",
											Namespace:       new("default"),
											Name:            new("test-pod"),
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
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
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
											Namespace:       new("default"),
											Name:            new("test-pod"),
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

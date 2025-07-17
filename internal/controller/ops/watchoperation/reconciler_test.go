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

package watchoperation

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	"github.com/crossplane/crossplane/internal/engine"
)

var errBoom = errors.New("boom")

// MockEngine implements the ControllerEngine interface for testing.
type MockEngine struct {
	MockStart        func(name string, o ...engine.ControllerOption) error
	MockStop         func(ctx context.Context, name string) error
	MockIsRunning    func(name string) bool
	MockGetWatches   func(name string) ([]engine.WatchID, error)
	MockStartWatches func(ctx context.Context, name string, ws ...engine.Watch) error
	MockStopWatches  func(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	MockGetCached    func() client.Client
	MockGetUncached  func() client.Client
}

func (m *MockEngine) Start(name string, o ...engine.ControllerOption) error {
	return m.MockStart(name, o...)
}

func (m *MockEngine) Stop(ctx context.Context, name string) error {
	return m.MockStop(ctx, name)
}

func (m *MockEngine) IsRunning(name string) bool {
	return m.MockIsRunning(name)
}

func (m *MockEngine) GetWatches(name string) ([]engine.WatchID, error) {
	return m.MockGetWatches(name)
}

func (m *MockEngine) StartWatches(ctx context.Context, name string, ws ...engine.Watch) error {
	return m.MockStartWatches(ctx, name, ws...)
}

func (m *MockEngine) StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error) {
	return m.MockStopWatches(ctx, name, ws...)
}

func (m *MockEngine) GetCached() client.Client {
	return m.MockGetCached()
}

func (m *MockEngine) GetUncached() client.Client {
	return m.MockGetUncached()
}

func (m *MockEngine) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func TestReconcile(t *testing.T) {
	type params struct {
		client client.Client
		engine ControllerEngine
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
		"NotFound": {
			reason: "Should return early if WatchOperation is not found",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				engine: &MockEngine{},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"GetError": {
			reason: "Should return an error if getting WatchOperation fails",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				engine: &MockEngine{},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"DeletedStopError": {
			reason: "Should return an error if stopping watched controller fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						now := metav1.Now()
						wo.SetDeletionTimestamp(&now)
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{
					MockStop: func(_ context.Context, _ string) error {
						return errBoom
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"DeletedRemoveFinalizerError": {
			reason: "Should return an error if removing finalizer fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						now := metav1.Now()
						wo.SetDeletionTimestamp(&now)
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockUpdate:       test.NewMockUpdateFn(errBoom),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{
					MockStop: func(_ context.Context, _ string) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"DeletedSuccess": {
			reason: "Should successfully delete WatchOperation and stop controller",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						now := metav1.Now()
						wo.SetDeletionTimestamp(&now)
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				engine: &MockEngine{
					MockStop: func(_ context.Context, _ string) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    nil,
			},
		},
		"AddFinalizerError": {
			reason: "Should return an error if adding finalizer fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockUpdate:       test.NewMockUpdateFn(errBoom),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"Paused": {
			reason: "Should return early if WatchOperation is paused",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.SetFinalizers([]string{finalizer})
						wo.SetAnnotations(map[string]string{
							"crossplane.io/paused": "true",
						})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
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
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockList:         test.NewMockListFn(errBoom),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"StartControllerError": {
			reason: "Should return an error if starting watched controller fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							ol.Items = []v1alpha1.Operation{}
							return nil
						}
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							ul.Items = []unstructured.Unstructured{}
							return nil
						}
						return errBoom
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{}
					},
					MockStart: func(_ string, _ ...engine.ControllerOption) error {
						return errBoom
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"StartWatchesError": {
			reason: "Should return an error if starting watches fails",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							ol.Items = []v1alpha1.Operation{}
							return nil
						}
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							ul.Items = []unstructured.Unstructured{}
							return nil
						}
						return errBoom
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{}
					},
					MockStart: func(_ string, _ ...engine.ControllerOption) error {
						return nil
					},
					MockStartWatches: func(_ context.Context, _ string, _ ...engine.Watch) error {
						return errBoom
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "Should successfully reconcile WatchOperation and start controller",
			params: params{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						wo := obj.(*v1alpha1.WatchOperation)
						wo.SetName("test-watch")
						wo.SetUID("test-uid")
						wo.SetFinalizers([]string{finalizer})
						wo.Spec.Watch = v1alpha1.WatchSpec{
							APIVersion: "v1",
							Kind:       "Pod",
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ol, ok := list.(*v1alpha1.OperationList); ok {
							ol.Items = []v1alpha1.Operation{}
							return nil
						}
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							ul.Items = []unstructured.Unstructured{}
							return nil
						}
						return errBoom
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				engine: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{}
					},
					MockStart: func(_ string, _ ...engine.ControllerOption) error {
						return nil
					},
					MockStartWatches: func(_ context.Context, _ string, _ ...engine.Watch) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "test-watch",
					},
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.client,
				WithControllerEngine(&engine.ControllerEngine{}))
			r.engine = tc.params.engine
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

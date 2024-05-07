/*
Copyright 2024 The Crossplane Authors.

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

package definition

import (
	"context"
	"reflect"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestEnqueueForCompositionRevisionFunc(t *testing.T) {
	type args struct {
		of    schema.GroupVersionKind
		list  func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error
		event runtimeevent.TypedCreateEvent[*v1.CompositionRevision]
	}
	type want struct {
		added []interface{}
	}

	dog := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Dog"}
	dogList := dog.GroupVersion().WithKind("DogList")

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "empty",
			args: args{
				of: dog,
				list: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
					// test parameters only here, not in the later tests for brevity.
					u, ok := list.(*kunstructured.UnstructuredList)
					if !ok {
						t.Errorf("list was not an UnstructuredList")
					} else if got := u.GroupVersionKind(); got != dogList {
						t.Errorf("list was not a DogList, got: %s", got)
					}
					if len(opts) != 0 {
						t.Errorf("unexpected list options: %#v", opts)
					}
					return nil
				},
				event: runtimeevent.TypedCreateEvent[*v1.CompositionRevision]{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
		},
		{
			name: "automatic management policy",
			args: args{
				of: dog,
				list: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					var obj1 composite.Unstructured
					obj1.SetNamespace("ns")
					obj1.SetName("obj1")
					policy := xpv1.UpdateAutomatic
					obj1.SetCompositionUpdatePolicy(&policy)
					obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

					list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

					return nil
				},
				event: runtimeevent.TypedCreateEvent[*v1.CompositionRevision]{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{
				added: []interface{}{reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: "ns",
					Name:      "obj1",
				}}},
			},
		},
		{
			name: "manual management policy",
			args: args{
				of: dog,
				list: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					var obj1 composite.Unstructured
					obj1.SetNamespace("ns")
					obj1.SetName("obj1")
					policy := xpv1.UpdateManual
					obj1.SetCompositionUpdatePolicy(&policy)
					obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

					list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

					return nil
				},
				event: runtimeevent.TypedCreateEvent[*v1.CompositionRevision]{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{},
		},
		{
			name: "other composition",
			args: args{
				of: dog,
				list: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					var obj1 composite.Unstructured
					obj1.SetNamespace("ns")
					obj1.SetName("obj1")
					policy := xpv1.UpdateAutomatic
					obj1.SetCompositionUpdatePolicy(&policy)
					obj1.SetCompositionReference(&corev1.ObjectReference{Name: "bernese"})

					list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

					return nil
				},
				event: runtimeevent.TypedCreateEvent[*v1.CompositionRevision]{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{},
		},
		{
			name: "multiple",
			args: args{
				of: dog,
				list: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					var obj1 composite.Unstructured
					obj1.SetNamespace("ns")
					obj1.SetName("obj1")
					automatic := xpv1.UpdateAutomatic
					obj1.SetCompositionUpdatePolicy(&automatic)
					obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

					obj2 := obj1.DeepCopy()
					obj2.SetName("obj2")

					obj3 := obj1.DeepCopy()
					obj3.SetName("obj3")
					obj3.SetCompositionReference(&corev1.ObjectReference{Name: "bernese"})

					obj4 := obj1.DeepCopy()
					obj4.SetName("obj4")
					manual := xpv1.UpdateManual
					obj4.SetCompositionUpdatePolicy(&manual)

					list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{
						obj1.Unstructured,
						obj2.Unstructured,
						obj3.Unstructured,
					}

					return nil
				},
				event: runtimeevent.TypedCreateEvent[*v1.CompositionRevision]{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{
				added: []interface{}{
					reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj1"}},
					reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj2"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := EnqueueForCompositionRevisionFunc(resource.CompositeKind(tt.args.of), tt.args.list, logging.NewNopLogger())
			q := rateLimitingQueueMock{}
			fn(context.TODO(), tt.args.event, &q)
			if got := q.added; !reflect.DeepEqual(got, tt.want.added) {
				t.Errorf("EnqueueForCompositionRevisionFunc(...)(ctx, event, queue) = %v, want %v", got, tt.want)
			}
		})
	}
}

type rateLimitingQueueMock struct {
	workqueue.RateLimitingInterface
	added []interface{}
}

func (f *rateLimitingQueueMock) Add(item interface{}) {
	f.added = append(f.added, item)
}

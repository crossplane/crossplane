/*
Copyright 2020 The Crossplane Authors.

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

package unstructured

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	nameWrapped   = "wrapped"
	nameUnwrapped = "unwrapped"

	errWrapped   = errors.New("unexpected Wrapped object")
	errUnwrapped = errors.New("unexpected Unwrapped object")
)

var _ client.Client = &WrapperClient{}

type Wrapped struct{ client.Object }

func (w *Wrapped) GetUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": nameWrapped,
		},
	}}
}

func NewWrapped() *Wrapped { return &Wrapped{} }

type WrappedList struct{ client.ObjectList }

func (w *WrappedList) GetUnstructuredList() *unstructured.UnstructuredList {
	u := NewWrapped().GetUnstructured()
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*u}}
}

func NewWrappedList() *WrappedList { return &WrappedList{} }

func NewUnwrapped() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": nameUnwrapped,
		},
	}}
}

func NewUnwrappedList() *unstructured.UnstructuredList {
	u := NewUnwrapped()
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*u}}
}

func TestGet(t *testing.T) {
	type args struct {
		ctx context.Context
		key client.ObjectKey
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Get(tc.args.ctx, tc.args.key, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Get(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestList(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.ObjectList
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
				u := &obj.(*unstructured.UnstructuredList).Items[0]
				if u.GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrappedList()},
		},
		"Wrapped": {
			c: &test.MockClient{MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
				u := &obj.(*unstructured.UnstructuredList).Items[0]
				if u.GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrappedList()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.List(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.List(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Create(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Create(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockDelete: test.NewMockDeleteFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockDelete: test.NewMockDeleteFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Delete(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Delete(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Update(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Update(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestPatch(t *testing.T) {
	type args struct {
		ctx   context.Context
		obj   client.Object
		patch client.Patch
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Patch(tc.args.ctx, tc.args.obj, tc.args.patch)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Patch(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestDeleteAllOf(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.DeleteAllOf(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.DeleteAllOf(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestStatusCreate(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
		sub client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockStatusCreate: test.NewMockSubResourceCreateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockStatusCreate: test.NewMockSubResourceCreateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Status().Create(tc.args.ctx, tc.args.obj, tc.args.sub)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Status().Create(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestStatusUpdate(t *testing.T) {
	type args struct {
		ctx context.Context
		obj client.Object
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Status().Update(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.Status().Update(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

func TestStatusPatch(t *testing.T) {
	type args struct {
		ctx   context.Context
		obj   client.Object
		patch client.Patch
	}
	cases := map[string]struct {
		c    client.Client
		args args
		want error
	}{
		"Unwrapped": {
			c: &test.MockClient{MockStatusPatch: test.NewMockSubResourcePatchFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameUnwrapped {
					return errWrapped
				}
				return nil
			})},
			args: args{obj: NewUnwrapped()},
		},
		"Wrapped": {
			c: &test.MockClient{MockStatusPatch: test.NewMockSubResourcePatchFn(nil, func(obj client.Object) error {
				if obj.(metav1.Object).GetName() != nameWrapped {
					return errUnwrapped
				}
				return nil
			})},
			args: args{obj: NewWrapped()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewClient(tc.c)
			got := c.Status().Patch(tc.args.ctx, tc.args.obj, tc.args.patch)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nc.StatusPatch(...): -want error, +got error:\n %s", diff)
			}
		})
	}
}

/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestAPIPatchingApplicator(t *testing.T) {
	errBoom := errors.New("boom")
	desired := &object{}
	desired.SetName("desired")

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot get object"),
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot create object"),
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			args: args{
				o:  &object{},
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   &object{},
				err: errBoom,
			},
		},
		"PatchError": {
			reason: "An error should be returned if we can't patch the object",
			c: &test.MockClient{
				MockGet:   test.NewMockGetFn(nil),
				MockPatch: test.NewMockPatchFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot patch object"),
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
		"Patched": {
			reason: "No error should be returned if we successfully patch an existing object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
				MockPatch: test.NewMockPatchFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIPatchingApplicator(tc.c)

			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestAPIUpdatingApplicator(t *testing.T) {
	errBoom := errors.New("boom")
	desired := &object{}
	desired.SetName("desired")

	current := &object{}
	current.SetName("current")

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot get object"),
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot create object"),
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			args: args{
				o:  &object{},
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   &object{},
				err: errBoom,
			},
		},
		"UpdateError": {
			reason: "An error should be returned if we can't update the object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(nil),
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot update object"),
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
		"Updated": {
			reason: "No error should be returned if we successfully update an existing object. If no ApplyOption is passed the existing should not be modified",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
					if diff := cmp.Diff(*desired, *o.(*object)); diff != "" {
						t.Errorf("r: -want, +got:\n%s", diff)
					}

					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIUpdatingApplicator(tc.c)

			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestManagedRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)

			err := api.RemoveFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)

			err := api.AddFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

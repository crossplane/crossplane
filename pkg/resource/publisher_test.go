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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

var (
	_ ManagedConnectionPublisher = &APISecretPublisher{}
	_ ManagedConnectionPublisher = PublisherChain{}
)

func TestPublisherChain(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
		c   ConnectionDetails
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		p    ManagedConnectionPublisher
		args args
		want error
	}{
		"EmptyChain": {
			p: PublisherChain{},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{},
				c:   ConnectionDetails{},
			},
			want: nil,
		},
		"SuccessfulPublisher": {
			p: PublisherChain{
				ManagedConnectionPublisherFn{
					PublishConnectionFn: func(_ context.Context, mg Managed, c ConnectionDetails) error {
						return nil
					},
					UnpublishConnectionFn: func(ctx context.Context, mg Managed, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{},
				c:   ConnectionDetails{},
			},
			want: nil,
		},
		"PublisherReturnsError": {
			p: PublisherChain{
				ManagedConnectionPublisherFn{
					PublishConnectionFn: func(_ context.Context, mg Managed, c ConnectionDetails) error {
						return errBoom
					},
					UnpublishConnectionFn: func(ctx context.Context, mg Managed, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{},
				c:   ConnectionDetails{},
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.p.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPISecretPublisher(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		mg  Managed
		c   ConnectionDetails
	}

	mgname := "coolmanaged"
	mgcsname := "coolmanagedsecret"
	mgcsdata := map[string][]byte{
		"cool":   []byte("data"),
		"cooler": []byte("notdata?"),
	}
	cddata := map[string][]byte{
		"cooler":  []byte("data"),
		"coolest": []byte("data"),
	}
	controller := true

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"ManagedSecretConflictError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetOwnerReferences([]metav1.OwnerReference{{
							UID:        types.UID("some-other-uuid"),
							Controller: &controller,
						}})
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: MockSchemeWith(&MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				c: ConnectionDetails{},
			},
			want: errors.Wrap(errors.Wrap(errors.New(errSecretConflict), "could not mutate object for update"), errCreateOrUpdateSecret),
		},
		"ManagedSecretUncontrolledError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						*o.(*corev1.Secret) = corev1.Secret{}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: MockSchemeWith(&MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				c: ConnectionDetails{},
			},
			want: errors.Wrap(errors.Wrap(errors.New(errSecretConflict), "could not mutate object for update"), errCreateOrUpdateSecret),
		},
		"SuccessfulCreate": {
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
							Controller: &controller,
						}})
						want.Data = cddata
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
		"SuccessfulUpdateEmptyManagedSecret": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetName(mgcsname)
						s.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
							Controller: &controller,
						}})
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
							Controller: &controller,
						}})
						want.Data = cddata
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
		"SuccessfulUpdatePopulatedManagedSecret": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetName(mgcsname)
						s.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
							Controller: &controller,
						}})
						s.Data = mgcsdata
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
							Controller: &controller,
						}})
						want.Data = map[string][]byte{
							"cool":    []byte("data"),
							"cooler":  []byte("data"),
							"coolest": []byte("data"),
						}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPISecretPublisher(tc.fields.client, tc.fields.typer)
			got := a.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
		})
	}
}

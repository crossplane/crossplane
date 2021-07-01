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

package claim

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ Binder               = &APIBinder{}
	_ ConnectionPropagator = &APIConnectionPropagator{}
)

func TestBind(t *testing.T) {
	errBoom := errors.New("boom")

	type fields struct {
		c client.Client
		t runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  resource.CompositeClaim
		cp  resource.Composite
	}

	cases := map[string]struct {
		reason    string
		fields    fields
		args      args
		want      error
		wantClaim resource.CompositeClaim
	}{
		"ReconcileXRCExtNameFromXR": {
			reason: "If existing XR already has an external-name, XRC's external-name should be set from it",
			fields: fields{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				t: fake.SchemeWith(&fake.Composite{}, &fake.CompositeClaim{}),
			},
			args: args{
				cm: &fake.CompositeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							meta.AnnotationKeyExternalName: "name-from-claim",
						},
					},
					CompositeResourceReferencer: fake.CompositeResourceReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Composite{}).Kind,
							Name:       "wat",
						},
					},
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wat",
						Annotations: map[string]string{
							meta.AnnotationKeyExternalName: "name-from-composite",
						},
					},
				},
			},
			wantClaim: &fake.CompositeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						meta.AnnotationKeyExternalName: "name-from-composite",
					},
				},
				CompositeResourceReferencer: fake.CompositeResourceReferencer{
					Ref: &corev1.ObjectReference{
						APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
						Kind:       fake.GVK(&fake.Composite{}).Kind,
						Name:       "wat",
					},
				},
			},
		},
		"CompositeRefConflict": {
			reason: "An error should be returned if the claim is bound to another composite resource",
			fields: fields{
				t: fake.SchemeWith(&fake.Composite{}),
			},
			args: args{
				cm: &fake.CompositeClaim{
					CompositeResourceReferencer: fake.CompositeResourceReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Composite{}).Kind,
							Name:       "who",
						},
					},
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wat",
					},
				},
			},
			want: errors.New(errBindClaimConflict),
		},
		"UpdateClaimError": {
			reason: "Errors updating the claim should be returned",
			fields: fields{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				t: fake.SchemeWith(&fake.Composite{}),
			},
			args: args{
				cm: &fake.CompositeClaim{
					CompositeResourceReferencer: fake.CompositeResourceReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Composite{}).Kind,
						},
					},
				},
				cp: &fake.Composite{},
			},
			want: errors.Wrap(errBoom, errUpdateClaim),
		},
		"ClaimRefConflict": {
			reason: "An error should be returned if the composite resource is bound to another claim",
			fields: fields{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				t: fake.SchemeWith(&fake.Composite{}, &fake.CompositeClaim{}),
			},
			args: args{
				cm: &fake.CompositeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wat",
					},
					CompositeResourceReferencer: fake.CompositeResourceReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Composite{}).Kind,
						},
					},
				},
				cp: &fake.Composite{
					ClaimReferencer: fake.ClaimReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.CompositeClaim{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.CompositeClaim{}).Kind,
							Name:       "who",
						},
					},
				},
			},
			want: errors.New(errBindCompositeConflict),
		},
		"UpdateCompositeError": {
			reason: "Errors updating the composite resource should be returned",
			fields: fields{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						if _, ok := obj.(*fake.Composite); ok {
							return errBoom
						}
						return nil
					}),
				},
				t: fake.SchemeWith(&fake.Composite{}, &fake.CompositeClaim{}),
			},
			args: args{
				cm: &fake.CompositeClaim{
					CompositeResourceReferencer: fake.CompositeResourceReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.Composite{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Composite{}).Kind,
						},
					},
				},
				cp: &fake.Composite{
					ClaimReferencer: fake.ClaimReferencer{
						Ref: &corev1.ObjectReference{
							APIVersion: fake.GVK(&fake.CompositeClaim{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.CompositeClaim{}).Kind,
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errUpdateComposite),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewAPIBinder(tc.fields.c, tc.fields.t)
			got := b.Bind(tc.args.ctx, tc.args.cm, tc.args.cp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("b.Bind(...): %s\n-want, +got:\n%s\n", tc.reason, diff)
			}
			if got != nil {
				return
			}

			// if no error, then assert the claim
			if diff := cmp.Diff(tc.wantClaim, tc.args.cm); diff != "" {
				t.Errorf("b.Bind(...): %s\n-wantClaim, +gotClaim:\n%s\n", tc.reason, diff)
			}
		})
	}

}

func TestPropagateConnection(t *testing.T) {
	errBoom := errors.New("boom")

	mgcsns := "coolnamespace"
	mgcsname := "coolmanagedsecret"
	mgcsdata := map[string][]byte{"cool": {1}}

	cmcsns := "coolnamespace"
	cmcsname := "coolclaimsecret"

	cp := &fake.Composite{
		ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
			Ref: &xpv1.SecretReference{Namespace: mgcsns, Name: mgcsname},
		},
	}

	cm := &fake.CompositeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: cmcsns},
		LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
			Ref: &xpv1.LocalSecretReference{Name: cmcsname},
		},
	}

	type fields struct {
		client resource.ClientApplicator
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx  context.Context
		to   resource.LocalConnectionSecretOwner
		from resource.ConnectionSecretOwner
	}
	type want struct {
		propagated bool
		err        error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ClaimDoesNotWantConnectionSecret": {
			reason: "The composite resource's secret should not be propagated if the claim does not want to write one",
			args: args{
				to:   &fake.CompositeClaim{},
				from: cp,
			},
			want: want{
				err: nil,
			},
		},
		"ManagedDoesNotExposeConnectionSecret": {
			reason: "The composite resource's secret should not be propagated if it does not have one",
			args: args{
				to:   cm,
				from: &fake.Managed{},
			},
			want: want{
				err: nil,
			},
		},
		"GetManagedSecretError": {
			reason: "Errors getting the composite resource's connection secret should be returned",
			fields: fields{
				client: resource.ClientApplicator{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				},
			},
			args: args{
				to:   cm,
				from: cp,
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"ManagedResourceDoesNotControlSecret": {
			reason: "The composite resource must control its connection secret before it can be propagated",
			fields: fields{
				client: resource.ClientApplicator{
					// Simulate getting a secret that is not controlled by the
					// composite resource by not modifying the secret passed to
					// the client, and not returning an error. We thus proceed
					// with our original empty secret, which has no controller
					// reference.
					Client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				},
			},
			args: args{
				to:   cm,
				from: cp,
			},
			want: want{
				err: errors.New(errSecretConflict),
			},
		},
		"ApplyClaimSecretError": {
			reason: "Errors applying the claim connection secret should be returned",
			fields: fields{
				client: resource.ClientApplicator{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						s := resource.ConnectionSecretFor(cp, fake.GVK(cp))
						*o.(*corev1.Secret) = *s
						return nil
					})},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error { return errBoom }),
				},
				typer: fake.SchemeWith(cp, cm),
			},
			args: args{
				to:   cm,
				from: cp,
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateOrUpdateSecret),
			},
		},
		"SuccessfulNoOp": {
			reason: "The claim secret should not be updated if it would not change",
			fields: fields{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// The managed secret has some data when we get it.
							s := resource.ConnectionSecretFor(cp, fake.GVK(cp))
							s.Data = mgcsdata

							*o.(*corev1.Secret) = *s
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
						// Simulate a no-op change by not allowing the update.
						return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
					}),
				},
				typer: fake.SchemeWith(cp, cm),
			},
			args: args{
				to:   cm,
				from: cp,
			},
			want: want{
				propagated: false,
			},
		},
		"SuccessfulPublish": {
			reason: "Successful propagation should update the claim secret with the appropriate values",
			fields: fields{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// The managed secret has some data when we get it.
							s := resource.ConnectionSecretFor(cp, fake.GVK(cp))
							s.Data = mgcsdata

							*o.(*corev1.Secret) = *s
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
						// Ensure the managed secret's data is copied to the
						// claim secret, and that the claim secret is annotated
						// to allow constant propagation from the managed
						// secret.
						want := resource.LocalConnectionSecretFor(cm, fake.GVK(cm))
						want.Data = mgcsdata
						if diff := cmp.Diff(want, o); diff != "" {
							t.Errorf("-want, +got: %s", diff)
						}

						return nil
					}),
				},
				typer: fake.SchemeWith(cp, cm),
			},
			args: args{
				to:   cm,
				from: cp,
			},
			want: want{
				propagated: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &APIConnectionPropagator{client: tc.fields.client, typer: tc.fields.typer}
			got, err := api.PropagateConnection(tc.args.ctx, tc.args.to, tc.args.from)
			if diff := cmp.Diff(tc.want.propagated, got); diff != "" {
				t.Errorf("\n%s\napi.PropagateConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\napi.PropagateConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

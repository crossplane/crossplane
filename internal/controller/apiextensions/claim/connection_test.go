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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ConnectionPropagator = &APIConnectionPropagator{}
)

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
							s := resource.ConnectionSecretFor(cp, schema.GroupVersionKind{})
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
						want := resource.LocalConnectionSecretFor(cm, schema.GroupVersionKind{})
						want.Data = mgcsdata
						if diff := cmp.Diff(want, o); diff != "" {
							t.Errorf("-want, +got:\n %s", diff)
						}

						return nil
					}),
				},
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
			api := &APIConnectionPropagator{client: tc.fields.client}
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

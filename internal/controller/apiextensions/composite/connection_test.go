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

package composite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ ConnectionDetailsFetcher = &SecretConnectionDetailsFetcher{}

func TestSecretConnectionDetailsFetcher(t *testing.T) {
	errBoom := errors.New("boom")
	sref := &xpv1.SecretReference{Name: "foo", Namespace: "bar"}
	s := &corev1.Secret{
		Data: map[string][]byte{
			"foo": []byte("a"),
			"bar": []byte("b"),
		},
	}

	type params struct {
		kube client.Client
	}
	type args struct {
		ctx context.Context
		o   resource.ConnectionSecretOwner
	}
	type want struct {
		conn managed.ConnectionDetails
		err  error
	}
	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"DoesNotPublish": {
			reason: "Should not fail if composed resource doesn't publish a connection secret",
			args: args{
				o: &fake.Composed{},
			},
		},
		"SecretNotPublishedYet": {
			reason: "Should not fail if composed resource has yet to publish the secret",
			params: params{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
			},
			args: args{
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				conn: nil,
			},
		},
		"SecretGetFailed": {
			reason: "Should fail if secret retrieval results in some error other than NotFound",
			params: params{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			args: args{
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"ClusterScopedOwner": {
			reason: "Should fetch all connection details from a connection secret owned by a cluster scoped resource - i.e. one with an empty namespace, and a populated secret ref namespace.",
			params: params{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if sobj, ok := obj.(*corev1.Secret); ok {
						if key.Name == sref.Name && key.Namespace == sref.Namespace {
							s.DeepCopyInto(sobj)
							return nil
						}
					}
					t.Errorf("wrong secret is queried")
					return errBoom
				}},
			},
			args: args{
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"foo": s.Data["foo"],
					"bar": s.Data["bar"],
				},
			},
		},
		"NamespacedOwner": {
			reason: "Should fetch all connection details from a connection secret owned by a namespaced resource - i.e. one with a populated namespace, and an empty secret ref namespace.",
			params: params{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					if sobj, ok := obj.(*corev1.Secret); ok {
						if key.Name == sref.Name && key.Namespace == "baz" {
							s.DeepCopyInto(sobj)
							return nil
						}
					}
					t.Errorf("wrong secret is queried")
					return errBoom
				}},
			},
			args: args{
				o: &fake.Composed{
					ObjectMeta:               metav1.ObjectMeta{Namespace: "baz"},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"foo": s.Data["foo"],
					"bar": s.Data["bar"],
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &SecretConnectionDetailsFetcher{client: tc.params.kube}
			conn, err := c.FetchConnection(tc.args.ctx, tc.args.o)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFetchFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

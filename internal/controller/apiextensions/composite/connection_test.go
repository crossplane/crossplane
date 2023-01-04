/*
Copyright 2022 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

var (
	_ managed.ConnectionDetailsFetcher = &SecretConnectionDetailsFetcher{}
	_ managed.ConnectionDetailsFetcher = ConnectionDetailsFetcherChain{}
)

func TestFetchConnection(t *testing.T) {
	errBoom := errors.New("boom")
	sref := &xpv1.SecretReference{Name: "foo", Namespace: "bar"}
	s := &corev1.Secret{
		Data: map[string][]byte{
			"foo": []byte("a"),
			"bar": []byte("b"),
		},
	}

	type args struct {
		kube client.Client
		o    resource.ConnectionSecretOwner
	}
	type want struct {
		conn managed.ConnectionDetails
		err  error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"DoesNotPublish": {
			reason: "Should not fail if composed resource doesn't publish a connection secret",
			args: args{
				o: &fake.Composed{},
			},
		},
		"SecretNotPublishedYet": {
			reason: "Should not fail if composed resource has yet to publish the secret",
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
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
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"Success": {
			reason: "Should fetch all connection details from the connection secret.",
			args: args{
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
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &SecretConnectionDetailsFetcher{client: tc.args.kube}
			conn, err := c.FetchConnection(context.Background(), tc.args.o)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFetchFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

/*
func TestExtractConnectionDetails(t *testing.T) {
	sref := &xpv1.SecretReference{Name: "foo", Namespace: "bar"}
	s := &corev1.Secret{
		Data: map[string][]byte{
			"foo": []byte("a"),
			"bar": []byte("b"),
		},
	}

	type args struct {
		kube client.Client
		o    resource.ConnectionSecretOwner
		cfg  []ExtractConfig
	}
	type want struct {
		conn managed.ConnectionDetails
		err  error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"DoesNotPublish": {
			reason: "Should not fail if composed resource doesn't publish a connection secret",
			args: args{
				o: &fake.Composed{},
			},
		},
		"SecretNotPublishedYet": {
			reason: "Should not fail if composed resource has yet to publish the secret",
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				cfg: []ExtractConfig{
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "bar",
						FromConnectionDetailKey: pointer.StringPtr("bar"),
					},
					{
						Type:  ConnectionDetailTypeFromValue,
						Name:  "fixed",
						Value: pointer.StringPtr("value"),
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"fixed": []byte("value"),
				},
			},
		},
		"SecretGetFailed": {
			reason: "Should fail if secret retrieval results in some error other than NotFound",
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"FetchConfigSuccess": {
			reason: "Should publish only the selected set of secret keys",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				cfg: []ExtractConfig{
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "bar",
						FromConnectionDetailKey: pointer.StringPtr("bar"),
					},
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "none",
						FromConnectionDetailKey: pointer.StringPtr("none"),
					},
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "convfoo",
						FromConnectionDetailKey: pointer.StringPtr("foo"),
					},
					{
						Type:  ConnectionDetailTypeFromValue,
						Name:  "fixed",
						Value: pointer.StringPtr("value"),
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"convfoo": s.Data["foo"],
					"bar":     s.Data["bar"],
					"fixed":   []byte("value"),
				},
			},
		},
		"NoFetchConfigSuccess": {
			reason: "Should publish all connection details from the connection secret if no FetchConfigs are supplied",
			args: args{
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
		"ConnectionDetailValueNotSet": {
			reason: "Should error if Value type value is not set",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				cfg: []ExtractConfig{
					{
						Type: ConnectionDetailTypeFromValue,
						Name: "missingvalue",
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailVal, v1.ConnectionDetailTypeFromValue),
			},
		},
		"ErrConnectionDetailFromConnectionSecretKeyNotSet": {
			reason: "Should error if ConnectionDetailFromConnectionSecretKey type FromConnectionSecretKey is not set",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				cfg: []ExtractConfig{
					{
						Type: ConnectionDetailTypeFromConnectionSecretKey,
						Name: "missing-key",
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailTypeFromConnectionSecretKey),
			},
		},
		"ErrConnectionDetailFromFieldPathNotSet": {
			reason: "Should error if ConnectionDetailFromFieldPath type FromFieldPath is not set",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				cfg: []ExtractConfig{
					{
						Type: ConnectionDetailTypeFromFieldPath,
						Name: "missing-path",
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailPath, v1.ConnectionDetailTypeFromFieldPath),
			},
		},
		"SuccessFieldPath": {
			reason: "Should publish only the selected set of secret keys",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				cfg: []ExtractConfig{
					{
						Type:          ConnectionDetailTypeFromFieldPath,
						Name:          "name",
						FromFieldPath: pointer.StringPtr("objectMeta.name"),
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"name": []byte("test"),
				},
			},
		},
		"SuccessFieldPathMarshal": {
			reason: "Should publish the secret keys as a JSON value",
			args: args{
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
				o: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 4,
					},
				},
				cfg: []ExtractConfig{
					{
						Type:          ConnectionDetailTypeFromFieldPath,
						Name:          "generation",
						FromFieldPath: pointer.StringPtr("objectMeta.generation"),
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"generation": []byte("4"),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &SecretConnectionDetailsFetcher{client: tc.args.kube}
			conn, err := c.FetchConnectionDetails(context.Background(), tc.args.o, tc.args.cfg...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
*/

func TestConnectionDetailType(t *testing.T) {
	fromVal := v1.ConnectionDetailTypeFromValue
	name := "coolsecret"
	value := "coolvalue"
	key := "coolkey"
	field := "coolfield"

	cases := map[string]struct {
		d    v1.ConnectionDetail
		want ConnectionDetailType
	}{
		"FromValueExplicit": {
			d:    v1.ConnectionDetail{Type: &fromVal},
			want: ConnectionDetailTypeFromValue,
		},
		"FromValueInferred": {
			d: v1.ConnectionDetail{
				Name:  &name,
				Value: &value,

				// Name and value trump key or field
				FromConnectionSecretKey: &key,
				FromFieldPath:           &field,
			},
			want: ConnectionDetailTypeFromValue,
		},
		"FromConnectionSecretKeyInferred": {
			d: v1.ConnectionDetail{
				Name:                    &name,
				FromConnectionSecretKey: &key,

				// From key trumps from field
				FromFieldPath: &field,
			},
			want: ConnectionDetailTypeFromConnectionSecretKey,
		},
		"FromFieldPathInferred": {
			d: v1.ConnectionDetail{
				Name:          &name,
				FromFieldPath: &field,
			},
			want: ConnectionDetailTypeFromFieldPath,
		},
		"DefaultToFromConnectionSecretKey": {
			d: v1.ConnectionDetail{
				Name: &name,
			},
			want: ConnectionDetailTypeFromConnectionSecretKey,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := connectionDetailType(tc.d)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("connectionDetailType(...): -want, +got\n%s", diff)
			}
		})
	}
}

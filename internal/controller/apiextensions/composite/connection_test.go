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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
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
		"Success": {
			reason: "Should fetch all connection details from the connection secret.",
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

func TestConnectionDetailsFetcherChain(t *testing.T) {
	errBoom := errors.New("boom")

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
		c      ConnectionDetailsFetcherChain
		args   args
		want   want
	}{
		"EmptyChain": {
			reason: "An empty chain should return empty connection details.",
			c:      ConnectionDetailsFetcherChain{},
			args: args{
				o: &fake.Composed{},
			},
			want: want{
				conn: managed.ConnectionDetails{},
			},
		},
		"SingleFetcherChain": {
			reason: "A chain of one fetcher should return only its connection details.",
			c: ConnectionDetailsFetcherChain{
				ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return managed.ConnectionDetails{"a": []byte("b")}, nil
				}),
			},
			args: args{
				o: &fake.Composed{},
			},
			want: want{
				conn: managed.ConnectionDetails{"a": []byte("b")},
			},
		},
		"FetcherError": {
			reason: "We should return errors from a chained fetcher.",
			c: ConnectionDetailsFetcherChain{
				ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return nil, errBoom
				}),
			},
			args: args{
				o: &fake.Composed{},
			},
			want: want{
				err: errBoom,
			},
		},
		"MultipleFetcherChain": {
			reason: "A chain of multiple fetchers should return all of their connection details, with later fetchers winning if there are duplicates.",
			c: ConnectionDetailsFetcherChain{
				ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return managed.ConnectionDetails{
						"a": []byte("a"),
						"b": []byte("b"),
						"c": []byte("c"),
					}, nil
				}),
				ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return managed.ConnectionDetails{
						"a": []byte("A"),
					}, nil
				}),
			},
			args: args{
				o: &fake.Composed{},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"a": []byte("A"),
					"b": []byte("b"),
					"c": []byte("c"),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			conn, err := tc.c.FetchConnection(tc.args.ctx, tc.args.o)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFetchFetchConnection(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractConnectionDetails(t *testing.T) {
	// errBoom := errors.New("boom")

	type args struct {
		cd   resource.Composed
		data managed.ConnectionDetails
		cfg  []ConnectionDetailExtractConfig
	}
	type want struct {
		conn managed.ConnectionDetails
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MissingNameError": {
			reason: "We should return an error if a connection detail is missing a name.",
			args: args{
				cfg: []ConnectionDetailExtractConfig{
					{
						// A nameless connection detail.
					},
				},
			},
			want: want{
				err: errors.New(errConnDetailName),
			},
		},
		"MissingValueError": {
			reason: "We should return an error if the fixed value is missing.",
			args: args{
				cfg: []ConnectionDetailExtractConfig{
					{
						Name: "cool-detail",
						Type: ConnectionDetailTypeFromValue,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailVal, ConnectionDetailTypeFromValue),
			},
		},
		"MissingConnectionSecretKeyError": {
			reason: "We should return an error if the connection secret key is missing.",
			args: args{
				cfg: []ConnectionDetailExtractConfig{
					{
						Name: "cool-detail",
						Type: ConnectionDetailTypeFromConnectionSecretKey,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailTypeFromConnectionSecretKey),
			},
		},
		"MissingFieldPathError": {
			reason: "We should return an error if the field path is missing.",
			args: args{
				cfg: []ConnectionDetailExtractConfig{
					{
						Name: "cool-detail",
						Type: ConnectionDetailTypeFromFieldPath,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailPath, v1.ConnectionDetailTypeFromFieldPath),
			},
		},
		"FetchConfigSuccess": {
			reason: "Should extract only the selected set of secret keys",
			args: args{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test",
						Generation: 4,
					},
				},
				data: managed.ConnectionDetails{
					"foo": []byte("a"),
					"bar": []byte("b"),
				},
				cfg: []ConnectionDetailExtractConfig{
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "bar",
						FromConnectionSecretKey: pointer.String("bar"),
					},
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "none",
						FromConnectionSecretKey: pointer.String("none"),
					},
					{
						Type:                    ConnectionDetailTypeFromConnectionSecretKey,
						Name:                    "convfoo",
						FromConnectionSecretKey: pointer.String("foo"),
					},
					{
						Type:  ConnectionDetailTypeFromValue,
						Name:  "fixed",
						Value: pointer.String("value"),
					},
					{
						Type:          ConnectionDetailTypeFromFieldPath,
						Name:          "name",
						FromFieldPath: pointer.String("objectMeta.name"),
					},
					{
						Type:          ConnectionDetailTypeFromFieldPath,
						Name:          "generation",
						FromFieldPath: pointer.String("objectMeta.generation"),
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"convfoo":    []byte("a"),
					"bar":        []byte("b"),
					"fixed":      []byte("value"),
					"name":       []byte("test"),
					"generation": []byte("4"),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			conn, err := ExtractConnectionDetails(tc.args.cd, tc.args.data, tc.args.cfg...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nExtractConnectionDetails(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nExtractConnectionDetails(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractConfigsFromTemplate(t *testing.T) {
	tfk := v1.ConnectionDetailTypeFromConnectionSecretKey

	type args struct {
		t *v1.ComposedTemplate
	}
	type want struct {
		cfgs []ConnectionDetailExtractConfig
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilTemplate": {
			reason: "A nil template should result in a nil slice of extract configs.",
			args: args{
				t: nil,
			},
			want: want{
				cfgs: nil,
			},
		},
		"ExplicitName": {
			reason: "When a template's connection details have an explicit name, we should use it.",
			args: args{
				t: &v1.ComposedTemplate{
					ConnectionDetails: []v1.ConnectionDetail{{
						Name:                    pointer.String("cool-detail"),
						Type:                    &tfk,
						FromConnectionSecretKey: pointer.String("cool-key"),
					}},
				},
			},
			want: want{
				cfgs: []ConnectionDetailExtractConfig{{
					Name:                    "cool-detail",
					Type:                    ConnectionDetailTypeFromConnectionSecretKey,
					FromConnectionSecretKey: pointer.String("cool-key"),
				}},
			},
		},
		"InferredName": {
			reason: "When a template's connection details does not have an explicit name and is of TypeFromConnectionSecretKey, we should infer the name from the connection secret key.",
			args: args{
				t: &v1.ComposedTemplate{
					ConnectionDetails: []v1.ConnectionDetail{{
						Type:                    &tfk,
						FromConnectionSecretKey: pointer.String("cool-key"),
					}},
				},
			},
			want: want{
				cfgs: []ConnectionDetailExtractConfig{{
					Name:                    "cool-key",
					Type:                    ConnectionDetailTypeFromConnectionSecretKey,
					FromConnectionSecretKey: pointer.String("cool-key"),
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfgs := ExtractConfigsFromComposedTemplate(tc.args.t)

			if diff := cmp.Diff(tc.want.cfgs, cfgs); diff != "" {
				t.Errorf("\n%s\nExtractConfigsFromTemplate(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

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

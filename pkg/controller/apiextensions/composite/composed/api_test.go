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

package composed

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

var errBoom = errors.New("boom")

func TestConfigure(t *testing.T) {

	tmpl, _ := json.Marshal(&fake.Managed{})

	type args struct {
		cp resource.Composite
		cd resource.Composed
		t  v1alpha1.ComposedTemplate
	}
	type want struct {
		cd  resource.Composed
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidTemplate": {
			reason: "Invalid template should not be accepted",
			args: args{
				cd: &fake.Composed{},
				t:  v1alpha1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("olala")}},
			},
			want: want{
				cd:  &fake.Composed{},
				err: errors.Wrap(errors.New("invalid character 'o' looking for beginning of value"), errUnmarshal),
			},
		},
		"Success": {
			reason: "Configuration should result in the right object with correct generateName",
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Name: "cp"}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1alpha1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd", GenerateName: "cp-"}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultConfigurator{}
			err := c.Configure(tc.args.cp, tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFetch(t *testing.T) {

	sref := &runtimev1alpha1.SecretReference{Name: "foo", Namespace: "bar"}
	s := &v1.Secret{
		Data: map[string][]byte{
			"foo": []byte("a"),
			"bar": []byte("b"),
		},
	}

	type args struct {
		kube client.Client
		cd   resource.Composed
		t    v1alpha1.ComposedTemplate
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
				cd: &fake.Composed{},
			},
		},
		"SecretNotPublishedYet": {
			reason: "Should not fail if composed resource has yet to publish the secret",
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
		},
		"SecretGetFailed": {
			reason: "Should fail if secret retrieval results in some error other than NotFound",
			args: args{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"Success": {
			reason: "Should publish only the selected set of secret keys",
			args: args{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if sobj, ok := obj.(*v1.Secret); ok {
						if key.Name == sref.Name && key.Namespace == sref.Namespace {
							s.DeepCopyInto(sobj)
							return nil
						}
					}
					t.Errorf("wrong secret is queried")
					return errBoom
				}},
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1alpha1.ComposedTemplate{ConnectionDetails: []v1alpha1.ConnectionDetail{
					{FromConnectionSecretKey: "bar"},
					{Name: pointer.StringPtr("convfoo"), FromConnectionSecretKey: "foo"},
					{FromConnectionSecretKey: "none"},
				}},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"convfoo": s.Data["foo"],
					"bar":     s.Data["bar"],
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &APIConnectionDetailsFetcher{client: tc.args.kube}
			conn, err := c.Fetch(context.Background(), tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

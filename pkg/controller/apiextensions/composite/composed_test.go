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

package composite

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
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/pkg/xcrd"
)

func TestRender(t *testing.T) {
	ctrl := true
	tmpl, _ := json.Marshal(&fake.Managed{})

	type args struct {
		ctx context.Context
		cp  resource.Composite
		cd  resource.Composed
		t   v1beta1.ComposedTemplate
	}
	type want struct {
		cd  resource.Composed
		err error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		args
		want
	}{
		"InvalidTemplate": {
			reason: "Invalid template should not be accepted",
			args: args{
				cd: &fake.Composed{},
				t:  v1beta1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("olala")}},
			},
			want: want{
				cd:  &fake.Composed{},
				err: errors.Wrap(errors.New("invalid character 'o' looking for beginning of value"), errUnmarshal),
			},
		},
		"NoLabel": {
			reason: "The name prefix label has to be set",
			args: args{
				cp: &fake.Composite{},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1beta1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd:  &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				err: errors.New(errNamePrefix),
			},
		},
		"DryRunError": {
			reason: "Errors dry-run creating the rendered resource to name it should be returned",
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)},
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					xcrd.LabelKeyNamePrefixForComposed: "ola",
					xcrd.LabelKeyClaimName:             "rola",
					xcrd.LabelKeyClaimNamespace:        "rolans",
				}}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{}},
				t:  v1beta1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ola-",
					Labels: map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "ola",
						xcrd.LabelKeyClaimName:             "rola",
						xcrd.LabelKeyClaimNamespace:        "rolans",
					},
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl}},
				}},
				err: errors.Wrap(errBoom, errName),
			},
		},
		"Success": {
			reason: "Configuration should result in the right object with correct generateName",
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(nil)},
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					xcrd.LabelKeyNamePrefixForComposed: "ola",
					xcrd.LabelKeyClaimName:             "rola",
					xcrd.LabelKeyClaimNamespace:        "rolans",
				}}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1beta1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					Name:         "cd",
					GenerateName: "ola-",
					Labels: map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "ola",
						xcrd.LabelKeyClaimName:             "rola",
						xcrd.LabelKeyClaimNamespace:        "rolans",
					},
					OwnerReferences: []metav1.OwnerReference{{Controller: &ctrl}},
				}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIDryRunRenderer(tc.client)
			err := r.Render(tc.args.ctx, tc.args.cp, tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
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
		t    v1beta1.ComposedTemplate
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
				t: v1beta1.ComposedTemplate{ConnectionDetails: []v1beta1.ConnectionDetail{
					{
						FromConnectionSecretKey: pointer.StringPtr("bar"),
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Value: pointer.StringPtr("value"),
					},
				}},
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
				t: v1beta1.ComposedTemplate{ConnectionDetails: []v1beta1.ConnectionDetail{
					{
						FromConnectionSecretKey: pointer.StringPtr("bar"),
					},
					{
						FromConnectionSecretKey: pointer.StringPtr("none"),
					},
					{
						Name:                    pointer.StringPtr("convfoo"),
						FromConnectionSecretKey: pointer.StringPtr("foo"),
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Value: pointer.StringPtr("value"),
					},
					{
						// Entries with only a name are silently ignored.
						Name: pointer.StringPtr("missingvalue"),
					},
					{
						// Entries with only a value are silently ignored.
						Value: pointer.StringPtr("missingname"),
					},
				}},
			},
			want: want{
				conn: managed.ConnectionDetails{
					"convfoo": s.Data["foo"],
					"bar":     s.Data["bar"],
					"fixed":   []byte("value"),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &APIConnectionDetailsFetcher{client: tc.args.kube}
			conn, err := c.FetchConnectionDetails(context.Background(), tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, conn); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsReady(t *testing.T) {
	type args struct {
		cd *composed.Unstructured
		t  v1beta1.ComposedTemplate
	}
	type want struct {
		ready bool
		err   error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoCustomCheck": {
			reason: "If no custom check is given, Ready condition should be used",
			args: args{
				cd: composed.New(composed.WithConditions(runtimev1alpha1.Available())),
			},
			want: want{
				ready: true,
			},
		},
		"ExplictNone": {
			reason: "If the only readiness check is explicitly 'None' the resource is always ready.",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: v1beta1.ReadinessCheckNone}}},
			},
			want: want{
				ready: true,
			},
		},
		"NonEmptyErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata..uid"}}},
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected '.' at position 9"), "cannot parse path %q", "metadata..uid"),
			},
		},
		"NonEmptyFalse": {
			reason: "If the field does not have value, NonEmpty check should return false",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata.uid"}}},
			},
			want: want{
				ready: false,
			},
		},
		"NonEmptyTrue": {
			reason: "If the field does have a value, NonEmpty check should return true",
			args: args{
				cd: composed.New(func(r *composed.Unstructured) {
					r.SetUID("olala")
				}),
				t: v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata.uid"}}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchStringErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata..uid"}}},
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected '.' at position 9"), "cannot parse path %q", "metadata..uid"),
			},
		},
		"MatchStringFalse": {
			reason: "If the value of the field does not match, it should return false",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata.uid", MatchString: "olala"}}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchStringTrue": {
			reason: "If the value of the field does match, it should return true",
			args: args{
				cd: composed.New(func(r *composed.Unstructured) {
					r.SetUID("olala")
				}),
				t: v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata.uid", MatchString: "olala"}}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchIntegerErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "metadata..uid"}}},
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected '.' at position 9"), "cannot parse path %q", "metadata..uid"),
			},
		},
		"MatchIntegerFalse": {
			reason: "If the value of the field does not match, it should return false",
			args: args{
				cd: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]interface{}{
						"spec": map[string]interface{}{
							"someNum": int64(6),
						},
					}
				}),
				t: v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "spec.someNum", MatchInteger: 5}}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchIntegerTrue": {
			reason: "If the value of the field does match, it should return true",
			args: args{
				cd: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]interface{}{
						"spec": map[string]interface{}{
							"someNum": int64(5),
						},
					}
				}),
				t: v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "spec.someNum", MatchInteger: 5}}},
			},
			want: want{
				ready: true,
			},
		},
		"UnknownType": {
			reason: "If unknown type is chosen, it should return an error",
			args: args{
				cd: composed.New(),
				t:  v1beta1.ComposedTemplate{ReadinessChecks: []v1beta1.ReadinessCheck{{Type: "Olala"}}},
			},
			want: want{
				err: errors.New("readiness check at index 0: an unknown type is chosen"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ready, err := IsReady(context.Background(), tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsReady(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ready, ready); diff != "" {
				t.Errorf("\n%s\nIsReady(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

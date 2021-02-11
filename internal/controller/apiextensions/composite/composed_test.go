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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestRender(t *testing.T) {
	ctrl := true
	tmpl, _ := json.Marshal(&fake.Managed{})

	type args struct {
		ctx context.Context
		cp  resource.Composite
		cd  resource.Composed
		t   v1.ComposedTemplate
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
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("olala")}},
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
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
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
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
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
				t:  v1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
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

	sref := &xpv1.SecretReference{Name: "foo", Namespace: "bar"}
	s := &corev1.Secret{
		Data: map[string][]byte{
			"foo": []byte("a"),
			"bar": []byte("b"),
		},
	}

	type args struct {
		kube client.Client
		cd   resource.Composed
		t    v1.ComposedTemplate
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
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						FromConnectionSecretKey: pointer.StringPtr("bar"),
						Type:                    v1.ConnectionDetailFromConnectionSecretKey,
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Type:  v1.ConnectionDetailValue,
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						FromConnectionSecretKey: pointer.StringPtr("bar"),
						Type:                    v1.ConnectionDetailFromConnectionSecretKey,
					},
					{
						FromConnectionSecretKey: pointer.StringPtr("none"),
						Type:                    v1.ConnectionDetailFromConnectionSecretKey,
					},
					{
						Name:                    pointer.StringPtr("convfoo"),
						FromConnectionSecretKey: pointer.StringPtr("foo"),
						Type:                    v1.ConnectionDetailFromConnectionSecretKey,
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Value: pointer.StringPtr("value"),
						Type:  v1.ConnectionDetailValue,
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Name: pointer.StringPtr("missingvalue"),
						Type: v1.ConnectionDetailValue,
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailVal, v1.ConnectionDetailValue),
			},
		},
		"ErrConnectionDetailNameNotSet": {
			reason: "Should error if Value type name is not set",
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Value: pointer.StringPtr("missingname"),
						Type:  v1.ConnectionDetailValue,
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailValue),
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Type: v1.ConnectionDetailFromConnectionSecretKey,
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailFromConnectionSecretKey),
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Type: v1.ConnectionDetailFromFieldPath,
						Name: pointer.StringPtr("missingname"),
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailPath, v1.ConnectionDetailFromFieldPath),
			},
		},
		"ErrConnectionDetailFromFieldPathNameNotSet": {
			reason: "Should error if ConnectionDetailFromFieldPath type Name is not set",
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Type:          v1.ConnectionDetailFromFieldPath,
						FromFieldPath: pointer.StringPtr("fieldpath"),
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailFromFieldPath),
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Name:          pointer.StringPtr("name"),
						FromFieldPath: pointer.StringPtr("objectMeta.name"),
						Type:          v1.ConnectionDetailFromFieldPath,
					},
				}},
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 4,
					},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Name:          pointer.StringPtr("generation"),
						FromFieldPath: pointer.StringPtr("objectMeta.generation"),
						Type:          v1.ConnectionDetailFromFieldPath,
					},
				}},
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
		t  v1.ComposedTemplate
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
				cd: composed.New(composed.WithConditions(xpv1.Available())),
			},
			want: want{
				ready: true,
			},
		},
		"ExplictNone": {
			reason: "If the only readiness check is explicitly 'None' the resource is always ready.",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: v1.ReadinessCheckNone}}},
			},
			want: want{
				ready: true,
			},
		},
		"NonEmptyErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata..uid"}}},
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected '.' at position 9"), "cannot parse path %q", "metadata..uid"),
			},
		},
		"NonEmptyFalse": {
			reason: "If the field does not have value, NonEmpty check should return false",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata.uid"}}},
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
				t: v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "NonEmpty", FieldPath: "metadata.uid"}}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchStringErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata..uid"}}},
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected '.' at position 9"), "cannot parse path %q", "metadata..uid"),
			},
		},
		"MatchStringFalse": {
			reason: "If the value of the field does not match, it should return false",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata.uid", MatchString: "olala"}}},
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
				t: v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchString", FieldPath: "metadata.uid", MatchString: "olala"}}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchIntegerErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "metadata..uid"}}},
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
				t: v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "spec.someNum", MatchInteger: 5}}},
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
				t: v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "MatchInteger", FieldPath: "spec.someNum", MatchInteger: 5}}},
			},
			want: want{
				ready: true,
			},
		},
		"UnknownType": {
			reason: "If unknown type is chosen, it should return an error",
			args: args{
				cd: composed.New(),
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: "Olala"}}},
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

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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestRejectMixedTemplates(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"Mixed": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							Name: pointer.StringPtr("cool"),
						},
					},
				},
			},
			want: errors.New(errMixed),
		},
		"Anonymous": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							// Unnamed.
						},
					},
				},
			},
			want: nil,
		},
		"Named": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.StringPtr("cool"),
						},
						{
							Name: pointer.StringPtr("cooler"),
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectMixedTemplates(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectMixedTemplates(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestRejectDuplicateNames(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"Unique": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.StringPtr("cool"),
						},
						{
							Name: pointer.StringPtr("cooler"),
						},
					},
				},
			},
			want: nil,
		},
		"Anonymous": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							// Unnamed.
						},
					},
				},
			},
			want: nil,
		},
		"Duplicates": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.StringPtr("cool"),
						},
						{
							Name: pointer.StringPtr("cool"),
						},
					},
				},
			},
			want: errors.New(errDuplicate),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectDuplicateNames(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectDuplicateNames(...): -want, +got:\n%s", diff)
			}
		})
	}
}

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

func TestAssociateByOrder(t *testing.T) {
	t0 := v1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("zero")}}
	t1 := v1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("one")}}
	t2 := v1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("two")}}

	r0 := corev1.ObjectReference{Name: "zero"}
	r1 := corev1.ObjectReference{Name: "one"}
	r2 := corev1.ObjectReference{Name: "two"}

	cases := map[string]struct {
		reason string
		t      []v1.ComposedTemplate
		r      []corev1.ObjectReference
		want   []TemplateAssociation
	}{
		"NoReferences": {
			reason: "When there are no references we should return templates associated with empty references.",
			t:      []v1.ComposedTemplate{t0, t1, t2},
			want: []TemplateAssociation{
				{Template: t0},
				{Template: t1},
				{Template: t2},
			},
		},
		"SomeReferences": {
			reason: "We should return all templates when there are fewer references than templates.",
			t:      []v1.ComposedTemplate{t0, t1, t2},
			r:      []corev1.ObjectReference{r0, r1},
			want: []TemplateAssociation{
				{Template: t0, Reference: r0},
				{Template: t1, Reference: r1},
				{Template: t2},
			},
		},
		"ExtraReferences": {
			reason: "When there are more references than templates they should be truncated.",
			t:      []v1.ComposedTemplate{t0, t1},
			r:      []corev1.ObjectReference{r0, r1, r2},
			want: []TemplateAssociation{
				{Template: t0, Reference: r0},
				{Template: t1, Reference: r1},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := AssociateByOrder(tc.t, tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nAssociateByOrder(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGarbageCollectingAssociator(t *testing.T) {
	errBoom := errors.New("boom")

	n0 := "zero"
	t0 := v1.ComposedTemplate{Name: &n0}

	r0 := corev1.ObjectReference{Name: n0}

	type args struct {
		ctx context.Context
		cr  resource.Composite
		ct  []v1.ComposedTemplate
	}

	type want struct {
		tas []TemplateAssociation
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"AnonymousTemplates": {
			reason: "We should fall back to associating templates with references by order if any template is not named.",
			args: args{
				cr: &fake.Composite{},
				ct: []v1.ComposedTemplate{t0, {Name: nil}},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0}, {Template: v1.ComposedTemplate{Name: nil}}},
			},
		},
		"ResourceNotFoundError": {
			reason: "Non-existent resources should be ignored.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0}},
			},
		},
		"GetResourceError": {
			reason: "Errors getting a referenced resource should be returned.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposed),
			},
		},
		"AnonymousResource": {
			reason: "We should fall back to associating templates with references by order if any resource is not annotated with its template name.",
			c: &test.MockClient{
				// Return an empty (and thus unannotated) composed resource.
				MockGet: test.NewMockGetFn(nil),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0, Reference: r0}},
			},
		},
		"AssociatedResource": {
			reason: "We should associate referenced resources by their template name annotation.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					SetCompositionResourceName(obj.(metav1.Object), n0)
					return nil
				}),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0, Reference: r0}},
			},
		},
		"UncontrolledResource": {
			reason: "We should not garbage collect a resource that we don't control.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					// The template used to create this resource is no longer known to us.
					SetCompositionResourceName(obj, "unknown")

					// This resource is not controlled by us.
					ctrl := true
					obj.SetOwnerReferences([]metav1.OwnerReference{{
						Controller: &ctrl,
						UID:        types.UID("who-dat"),
					}})
					return nil
				}),
			},
			args: args{
				cr: &fake.Composite{
					ObjectMeta:                  metav1.ObjectMeta{UID: types.UID("very-unique")},
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0}},
			},
		},
		"GarbageCollectionError": {
			reason: "We should return errors encountered while garbage collecting a composed resource.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					// The template used to create this resource is no longer known to us.
					SetCompositionResourceName(obj, "unknown")
					return nil
				}),
				MockDelete: test.NewMockDeleteFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				err: errors.Wrap(errBoom, errGCComposed),
			},
		},
		"GarbageCollectedResource": {
			reason: "We should not return a resource that we successfully garbage collect.",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					// The template used to create this resource is no longer known to us.
					SetCompositionResourceName(obj, "unknown")
					return nil
				}),
				MockDelete: test.NewMockDeleteFn(nil),
			},
			args: args{
				cr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{Refs: []corev1.ObjectReference{r0}},
				},
				ct: []v1.ComposedTemplate{t0},
			},
			want: want{
				tas: []TemplateAssociation{{Template: t0}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewGarbageCollectingAssociator(tc.c)
			got, err := a.AssociateTemplates(tc.args.ctx, tc.args.cr, tc.args.ct)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAssociateTemplates(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.tas, got); diff != "" {
				t.Errorf("\n%s\nAssociateTemplates(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	fromKey := v1.ConnectionDetailTypeFromConnectionSecretKey
	fromVal := v1.ConnectionDetailTypeFromValue
	fromField := v1.ConnectionDetailTypeFromFieldPath

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
						Type:                    &fromKey,
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Type:  &fromVal,
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
						Type:                    &fromKey,
					},
					{
						FromConnectionSecretKey: pointer.StringPtr("none"),
						Type:                    &fromKey,
					},
					{
						Name:                    pointer.StringPtr("convfoo"),
						FromConnectionSecretKey: pointer.StringPtr("foo"),
						Type:                    &fromKey,
					},
					{
						Name:  pointer.StringPtr("fixed"),
						Value: pointer.StringPtr("value"),
						Type:  &fromVal,
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
						Type: &fromVal,
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailVal, v1.ConnectionDetailTypeFromValue),
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
						Type:  &fromVal,
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailTypeFromValue),
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
						Type: &fromKey,
					},
				}},
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
				cd: &fake.Composed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: sref},
				},
				t: v1.ComposedTemplate{ConnectionDetails: []v1.ConnectionDetail{
					{
						Type: &fromField,
						Name: pointer.StringPtr("missingname"),
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailPath, v1.ConnectionDetailTypeFromFieldPath),
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
						Type:          &fromField,
						FromFieldPath: pointer.StringPtr("fieldpath"),
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtConnDetailKey, v1.ConnectionDetailTypeFromFieldPath),
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
						Type:          &fromField,
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
						Type:          &fromField,
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

func TestConnectionDetailType(t *testing.T) {
	fromVal := v1.ConnectionDetailTypeFromValue
	name := "coolsecret"
	value := "coolvalue"
	key := "coolkey"
	field := "coolfield"

	cases := map[string]struct {
		d    v1.ConnectionDetail
		want v1.ConnectionDetailType
	}{
		"FromValueExplicit": {
			d:    v1.ConnectionDetail{Type: &fromVal},
			want: v1.ConnectionDetailTypeFromValue,
		},
		"FromValueInferred": {
			d: v1.ConnectionDetail{
				Name:  &name,
				Value: &value,

				// Name and value trump key or field
				FromConnectionSecretKey: &key,
				FromFieldPath:           &field,
			},
			want: v1.ConnectionDetailTypeFromValue,
		},
		"FromConnectionSecretKeyInferred": {
			d: v1.ConnectionDetail{
				Name:                    &name,
				FromConnectionSecretKey: &key,

				// From key trumps from field
				FromFieldPath: &field,
			},
			want: v1.ConnectionDetailTypeFromConnectionSecretKey,
		},
		"FromFieldPathInferred": {
			d: v1.ConnectionDetail{
				Name:          &name,
				FromFieldPath: &field,
			},
			want: v1.ConnectionDetailTypeFromFieldPath,
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
				t:  v1.ComposedTemplate{ReadinessChecks: []v1.ReadinessCheck{{Type: v1.ReadinessCheckTypeNone}}},
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
					r.Object = map[string]any{
						"spec": map[string]any{
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
					r.Object = map[string]any{
						"spec": map[string]any{
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

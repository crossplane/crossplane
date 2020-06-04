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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

var errBoom = errors.New("boom")

func TestPublishConnection(t *testing.T) {
	errBoom := errors.New("boom")

	owner := &fake.MockConnectionSecretOwner{
		Ref: &runtimev1alpha1.SecretReference{
			Namespace: "coolnamespace",
			Name:      "coolsecret",
		},
	}

	type args struct {
		applicator resource.Applicator
		o          resource.ConnectionSecretOwner
		filter     []string
		c          managed.ConnectionDetails
	}

	cases := map[string]struct {
		reason string
		args   args
		err    error
	}{
		"ResourceDoesNotPublishSecret": {
			reason: "A managed resource with a nil GetWriteConnectionSecretToReference should not publish a secret",
			args: args{
				o: &fake.MockConnectionSecretOwner{},
			},
		},
		"ApplyError": {
			reason: "An error applying the connection secret should be returned",
			args: args{
				applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error { return errBoom }),
				o:          owner,
			},
			err: errors.Wrap(errBoom, errApplySecret),
		},
		"Success": {
			reason: "A successful application of the connection secret should result in no error",
			args: args{
				applicator: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
					want := resource.ConnectionSecretFor(owner, owner.GetObjectKind().GroupVersionKind())
					want.Data = managed.ConnectionDetails{"onlyme": {41}}
					if diff := cmp.Diff(want, o); diff != "" {
						t.Errorf("-want, +got:\n%s", diff)
					}
					return nil
				}),
				o:      owner,
				c:      managed.ConnectionDetails{"cool": {42}, "onlyme": {41}},
				filter: []string{"onlyme"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &APIFilteredSecretPublisher{tc.args.applicator, tc.args.filter}
			got := a.PublishConnection(context.Background(), tc.args.o, tc.args.c)
			if diff := cmp.Diff(tc.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPublish(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConfigure(t *testing.T) {
	cs := fake.ConnectionSecretWriterTo{Ref: &runtimev1alpha1.SecretReference{
		Name:      "foo",
		Namespace: "bar",
	}}
	rp := fake.Reclaimer{Policy: runtimev1alpha1.ReclaimDelete}
	cp := &fake.Composite{
		ObjectMeta:               metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
		Reclaimer:                rp,
		ConnectionSecretWriterTo: cs,
	}

	type args struct {
		kube client.Client
		cp   resource.Composite
		comp *v1alpha1.Composition
	}
	type want struct {
		cp  resource.Composite
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"AlreadyFilled": {
			reason: "Should be no-op if reclaim policy and connection secret namespace is already filled",
			args:   args{cp: cp},
			want:   want{cp: cp},
		},
		"ReclaimPolicyMissing": {
			reason: "Should fill reclaim policy if missing",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
				cp: &fake.Composite{
					ObjectMeta:               cp.ObjectMeta,
					ConnectionSecretWriterTo: cs,
				},
				comp: &v1alpha1.Composition{Spec: v1alpha1.CompositionSpec{ReclaimPolicy: rp.Policy}},
			},
			want: want{cp: cp},
		},
		"ConnectionSecretRefMissing": {
			reason: "Should fill connection secret ref if missing",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
					Reclaimer:  rp,
				},
				comp: &v1alpha1.Composition{
					Spec: v1alpha1.CompositionSpec{WriteConnectionSecretsToNamespace: cs.Ref.Namespace},
				},
			},
			want: want{cp: cp},
		},
		"UpdateFailed": {
			reason: "Should fail if kube update failed",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
				},
				comp: &v1alpha1.Composition{
					Spec: v1alpha1.CompositionSpec{
						WriteConnectionSecretsToNamespace: cs.Ref.Namespace,
						ReclaimPolicy:                     rp.Policy,
					},
				},
			},
			want: want{
				cp:  cp,
				err: errors.Wrap(errBoom, errUpdateComposite),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &APIConfigurator{client: tc.args.kube}
			err := c.Configure(context.Background(), tc.args.cp, tc.args.comp)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelectorResolver(t *testing.T) {
	a, k := schema.EmptyObjectKind.GroupVersionKind().ToAPIVersionAndKind()
	tref := v1alpha1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1alpha1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.CompositionSpec{
			From: tref,
		},
	}
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"select": "me"}}

	type args struct {
		kube client.Client
		cp   resource.Composite
	}
	type want struct {
		cp  resource.Composite
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"AlreadyResolved": {
			reason: "Should be no-op if the composition selector is already resolved",
			args: args{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
		"ListFailed": {
			reason: "Should fail if List query fails",
			args: args{
				kube: &test.MockClient{MockList: test.NewMockListFn(errBoom)},
				cp:   &fake.Composite{},
			},
			want: want{
				cp:  &fake.Composite{},
				err: errors.Wrap(errBoom, errListCompositions),
			},
		},
		"NoneCompatible": {
			reason: "Should fail if it cannot find a compatible Composition",
			args: args{
				kube: &test.MockClient{MockList: test.NewMockListFn(nil)},
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: sel},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: sel},
				},
				err: errors.New(errNoCompatibleComposition),
			},
		},
		"SelectedTheCompatibleOne": {
			reason: "Should select the one that is compatible",
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						compList := &v1alpha1.CompositionList{
							Items: []v1alpha1.Composition{
								{
									Spec: v1alpha1.CompositionSpec{
										From: v1alpha1.TypeReference{APIVersion: "foreign", Kind: "tome"},
									},
								},
								*comp,
							},
						}
						if list, ok := obj.(*v1alpha1.CompositionList); ok {
							compList.DeepCopyInto(list)
							return nil
						}
						t.Errorf("wrong query")
						return nil
					}},
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: sel},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
					CompositionSelector:   fake.CompositionSelector{Sel: sel},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPISelectorResolver(tc.args.kube)
			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIDefaultCompositionSelector(t *testing.T) {
	a, k := schema.EmptyObjectKind.GroupVersionKind().ToAPIVersionAndKind()
	tref := v1alpha1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1alpha1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.CompositionSpec{
			From: tref,
		},
	}
	type args struct {
		kube   client.Client
		defRef v1.ObjectReference
		cp     resource.Composite
	}
	type want struct {
		cp  resource.Composite
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"AlreadyResolved": {
			reason: "Should be no-op if a composition is already selected",
			args: args{
				defRef: v1.ObjectReference{},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
		"SelectorInPlace": {
			reason: "Should be no-op if a composition selector is in place",
			args: args{
				defRef: v1.ObjectReference{},
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}},
				},
			},
		},
		"NoDefault": {
			reason: "Should be no-op if no default is given in definition",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{},
			},
		},
		"GetDefinitionFailed": {
			reason: "Should return error if InfraDef cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetInfraDef),
				cp:  &fake.Composite{},
			},
		},
		"GetCompositionFailed": {
			reason: "Should return error if default composition cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{DefaultCompositionRef: &v1.ObjectReference{}}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							return errBoom
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposition),
				cp:  &fake.Composite{},
			},
		},
		"CompositionNotCompatible": {
			reason: "Should fail if the default composition is not compatible",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{DefaultCompositionRef: &v1.ObjectReference{}}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							c := &v1alpha1.Composition{Spec: v1alpha1.CompositionSpec{From: v1alpha1.TypeReference{APIVersion: "ran", Kind: "dom"}}}
							c.DeepCopyInto(cr)
							return nil
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.New(errDefaultNotCompatible),
				cp:  &fake.Composite{},
			},
		},
		"Success": {
			reason: "Successfully set the default composition reference",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{DefaultCompositionRef: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							comp.DeepCopyInto(cr)
							return nil
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPIDefaultCompositionSelector(tc.args.kube, tc.args.defRef)
			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIEnforcedCompositionSelector(t *testing.T) {
	a, k := schema.EmptyObjectKind.GroupVersionKind().ToAPIVersionAndKind()
	tref := v1alpha1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1alpha1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.CompositionSpec{
			From: tref,
		},
	}
	type args struct {
		kube   client.Client
		defRef v1.ObjectReference
		cp     resource.Composite
	}
	type want struct {
		cp  resource.Composite
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"GetDefinitionFailed": {
			reason: "Should return error if InfraDef cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetInfraDef),
				cp:  &fake.Composite{},
			},
		},
		"NoEnforced": {
			reason: "Should be no-op if no enforced composition ref is given in definition",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{},
			},
		},
		"EnforcedAlreadySet": {
			reason: "Should be no-op if enforced composition reference is already set",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						if d, ok := obj.(*v1alpha1.InfrastructureDefinition); ok {
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{EnforcedCompositionRef: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)}}
							withRef.DeepCopyInto(d)
							return nil
						}
						return errBoom
					},
				},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
		"GetCompositionFailed": {
			reason: "Should return error if default composition cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{EnforcedCompositionRef: &v1.ObjectReference{}}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							return errBoom
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposition),
				cp:  &fake.Composite{},
			},
		},
		"CompositionNotCompatible": {
			reason: "Should fail if the default composition is not compatible",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{EnforcedCompositionRef: &v1.ObjectReference{}}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							c := &v1alpha1.Composition{Spec: v1alpha1.CompositionSpec{From: v1alpha1.TypeReference{APIVersion: "ran", Kind: "dom"}}}
							c.DeepCopyInto(cr)
							return nil
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.New(errEnforcedNotCompatible),
				cp:  &fake.Composite{},
			},
		},
		"Success": {
			reason: "Successfully set the default composition reference",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{EnforcedCompositionRef: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							comp.DeepCopyInto(cr)
							return nil
						}
						return nil
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
		"SuccessOverride": {
			reason: "Successfully set the default composition reference even if another one was set",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						switch cr := obj.(type) {
						case *v1alpha1.InfrastructureDefinition:
							withRef := &v1alpha1.InfrastructureDefinition{Spec: v1alpha1.InfrastructureDefinitionSpec{EnforcedCompositionRef: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1alpha1.Composition:
							comp.DeepCopyInto(cr)
							return nil
						}
						return nil
					},
				},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &v1.ObjectReference{Name: "ola"}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: meta.ReferenceTo(comp, v1alpha1.CompositionGroupVersionKind)},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPIEnforcedCompositionSelector(tc.args.kube, tc.args.defRef)
			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

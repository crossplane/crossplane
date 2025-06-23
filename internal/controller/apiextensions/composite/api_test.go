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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestPublishConnection(t *testing.T) {
	errBoom := errors.New("boom")

	owner := &fake.MockConnectionSecretOwner{
		WriterTo: &xpv1.SecretReference{
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

	type want struct {
		published bool
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
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
				applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error { return errBoom }),
				o:          owner,
			},
			want: want{
				err: errors.Wrap(errBoom, errApplySecret),
			},
		},
		"SuccessfulNoOp": {
			reason: "If application would be a no-op we should not publish a secret.",
			args: args{
				applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
					// Simulate a no-op change by not allowing the update.
					return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
				}),
				o:      owner,
				c:      managed.ConnectionDetails{"cool": {42}, "onlyme": {41}},
				filter: []string{"onlyme"},
			},
			want: want{
				published: false,
			},
		},
		"SuccessfulPublish": {
			reason: "If the secret changed we should publish it.",
			args: args{
				applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
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
			want: want{
				published: true,
			},
		},
		"SuccessfulPublishAllWithEmptyList": {
			reason: "We should publish all keys if the filter is empty.",
			args: args{
				applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
					want := resource.ConnectionSecretFor(owner, owner.GetObjectKind().GroupVersionKind())
					want.Data = managed.ConnectionDetails{"cool": {42}, "onlyme": {41}}
					if diff := cmp.Diff(want, o); diff != "" {
						t.Errorf("-want, +got:\n%s", diff)
					}
					return nil
				}),
				o:      owner,
				c:      managed.ConnectionDetails{"cool": {42}, "onlyme": {41}},
				filter: []string{},
			},
			want: want{
				published: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &APIFilteredSecretPublisher{tc.args.applicator, tc.args.filter}

			got, err := a.PublishConnection(context.Background(), tc.args.o, tc.args.c)
			if diff := cmp.Diff(tc.want.published, got); diff != "" {
				t.Errorf("\n%s\nPublish(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPublish(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFetchRevision(t *testing.T) {
	errBoom := errors.New("boom")
	manual := xpv1.UpdateManual
	uid := types.UID("no-you-id")
	ctrl := true

	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-composition",
			UID:  uid,
		},
	}

	// We don't own this revision.
	rev3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + "-jfdm2",
		},
	}

	// The latest revision.
	rev2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + "-dl2nd",
			Labels: map[string]string{
				v1.LabelCompositionHash: comp.Hash(),
			},
			OwnerReferences: []metav1.OwnerReference{{
				UID:                comp.GetUID(),
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 2},
	}

	// An older revision
	rev1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + "-mdk12",
			Labels: map[string]string{
				v1.LabelCompositionHash: "I'm different!",
			},
			OwnerReferences: []metav1.OwnerReference{{
				UID:                comp.GetUID(),
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 1},
	}

	type args struct {
		ctx context.Context
		cr  resource.Composite
	}

	type want struct {
		rev *v1.CompositionRevision
		err error
	}

	cases := map[string]struct {
		reason string
		client client.Client
		args   args
		want   want
	}{
		"GetCompositionRevisionError": {
			reason: "We should wrap and return errors encountered getting the CompositionRevision.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					CompositionRevisionReferencer: fake.CompositionRevisionReferencer{Ref: &corev1.LocalObjectReference{}},
					CompositionUpdater:            fake.CompositionUpdater{Policy: &manual},
				},
			},
			want: want{
				rev: &v1.CompositionRevision{},
				err: errors.Wrap(errBoom, errGetCompositionRevision),
			},
		},
		"UpdateManual": {
			reason: "When we're using the manual update policy and a revision reference is set we should return that revision as a composition.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					*obj.(*v1.CompositionRevision) = *rev3
					return nil
				}),
			},
			args: args{
				cr: &fake.Composite{
					CompositionRevisionReferencer: fake.CompositionRevisionReferencer{Ref: &corev1.LocalObjectReference{}},
					CompositionUpdater:            fake.CompositionUpdater{Policy: &manual},
				},
			},
			want: want{
				rev: rev3,
			},
		},
		"GetCompositionError": {
			reason: "We should wrap and return errors encountered getting the Composition.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposition),
			},
		},
		"ListCompositionRevisionsError": {
			reason: "We should wrap and return errors encountered listing CompositionRevisions.",
			client: &test.MockClient{
				MockGet:  test.NewMockGetFn(nil),
				MockList: test.NewMockListFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errListCompositionRevisions), errFetchCompositionRevision),
			},
		},
		"NoCompositionRevisionsError": {
			reason: "We should return an error if we don't find any suitable CompositionRevisions.",
			client: &test.MockClient{
				MockGet:  test.NewMockGetFn(nil),
				MockList: test.NewMockListFn(nil),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.New(errNoCompatibleCompositionRevision),
			},
		},
		"AlreadyAtLatestRevision": {
			reason: "We should return the latest revision without updating our reference if we already reference it.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					*obj.(*v1.Composition) = *comp
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
						Items: []v1.CompositionRevision{
							// We should ignore this revision because it does not have
							// our composition above as its controller reference.
							*rev3,

							// This revision is owned by our composition, and is the
							// latest revision.
							*rev2,

							// This revision is owned by our composition, but is not the
							// latest revision.
							*rev1,
						},
					}
					return nil
				}),
				// This should not be called.
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{
						Ref: &corev1.ObjectReference{Name: comp.GetName()},
					},
					// We're already using the latest revision.
					CompositionRevisionReferencer: fake.CompositionRevisionReferencer{
						Ref: &corev1.LocalObjectReference{Name: rev2.GetName()},
					},
				},
			},
			want: want{
				rev: rev2,
			},
		},
		"NoRevisionSet": {
			reason: "We should return the latest revision and update our reference if none is set.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					*obj.(*v1.Composition) = *comp
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
						Items: []v1.CompositionRevision{
							// This revision is owned by our composition, and is the
							// latest revision.
							*rev2,
						},
					}
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
					want := &fake.Composite{
						CompositionReferencer: fake.CompositionReferencer{
							Ref: &corev1.ObjectReference{Name: comp.GetName()},
						},
						CompositionRevisionReferencer: fake.CompositionRevisionReferencer{
							Ref: &corev1.LocalObjectReference{
								Name: rev2.GetName(),
							},
						},
						CompositionUpdater: fake.CompositionUpdater{Policy: &manual},
					}
					if diff := cmp.Diff(want, obj); diff != "" {
						t.Errorf("Apply(): -want, +got: %s", diff)
					}
					return nil
				}),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{
						Ref: &corev1.ObjectReference{Name: comp.GetName()},
					},
					// We want to set a reference when none exists, even with a
					// manual update policy.
					CompositionUpdater: fake.CompositionUpdater{Policy: &manual},
				},
			},
			want: want{
				rev: rev2,
			},
		},
		"OutdatedRevisionSet": {
			reason: "We should return the latest revision and update our reference if an outdated revision is referenced.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					*obj.(*v1.Composition) = *comp
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
						Items: []v1.CompositionRevision{
							// This revision is owned by our composition, and is the
							// latest revision.
							*rev2,

							// This is an outdated revision.
							*rev1,
						},
					}
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
					// Ensure we were updated to reference the latest CompositionRevision.
					want := &fake.Composite{
						CompositionReferencer: fake.CompositionReferencer{
							Ref: &corev1.ObjectReference{Name: comp.GetName()},
						},
						CompositionRevisionReferencer: fake.CompositionRevisionReferencer{
							Ref: &corev1.LocalObjectReference{
								Name: rev2.GetName(),
							},
						},
					}
					if diff := cmp.Diff(want, obj); diff != "" {
						t.Errorf("Apply(): -want, +got: %s", diff)
					}
					return nil
				}),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{
						Ref: &corev1.ObjectReference{Name: comp.GetName()},
					},
					// We reference the outdated revision.
					CompositionRevisionReferencer: fake.CompositionRevisionReferencer{
						Ref: &corev1.LocalObjectReference{
							Name: rev1.GetName(),
						},
					},
				},
			},
			want: want{
				rev: rev2,
			},
		},
		"SetRevisionError": {
			reason: "We should return the latest revision and update our reference if none is set.",
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					*obj.(*v1.Composition) = *comp
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
						Items: []v1.CompositionRevision{
							// This revision is owned by our composition, and is the
							// latest revision.
							*rev2,
						},
					}
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				cr: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{
						Ref: &corev1.ObjectReference{Name: comp.GetName()},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdate),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := NewAPIRevisionFetcher(tc.client)

			got, err := f.Fetch(tc.args.ctx, tc.args.cr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.Fetch(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rev, got); diff != "" {
				t.Errorf("%s\nf.Fetch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConfigure(t *testing.T) {
	errBoom := errors.New("boom")

	cs := fake.ConnectionSecretWriterTo{Ref: &xpv1.SecretReference{
		Name:      "foo",
		Namespace: "bar",
	}}
	cp := &fake.Composite{
		ObjectMeta:               metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
		ConnectionSecretWriterTo: cs,
	}

	type args struct {
		kube client.Client
		cp   resource.Composite
		rev  *v1.CompositionRevision
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
		"NotCompatible": {
			reason: "Should return error if given composition is not compatible",
			args: args{
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						CompositeTypeRef: v1.TypeReference{APIVersion: "ola/crossplane.io", Kind: "olala"},
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp:  &fake.Composite{},
				err: errors.New(errCompositionNotCompatible),
			},
		},
		"AlreadyFilled": {
			reason: "Should be no-op if connection secret namespace is already filled",
			args:   args{cp: cp, rev: &v1.CompositionRevision{}},
			want:   want{cp: cp},
		},
		"ConnectionSecretRefMissing": {
			reason: "Should fill connection secret ref if missing",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
				},
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{WriteConnectionSecretsToNamespace: &cs.Ref.Namespace},
				},
			},
			want: want{cp: cp},
		},
		"NilWriteConnectionSecretsToNamespace": {
			reason: "Should not fill connection secret ref if composition does not have WriteConnectionSecretsToNamespace",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
				},
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{},
				},
			},
			want: want{cp: &fake.Composite{
				ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
			}},
		},
		"UpdateFailed": {
			reason: "Should fail if kube update failed",
			args: args{
				kube: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{UID: types.UID(cs.Ref.Name)},
				},
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						WriteConnectionSecretsToNamespace: &cs.Ref.Namespace,
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
			c := &APIConfigurator{client: tc.kube}

			err := c.Configure(context.Background(), tc.args.cp, tc.rev)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelectorResolver(t *testing.T) {
	errBoom := errors.New("boom")

	a, k := schema.EmptyObjectKind.GroupVersionKind().ToAPIVersionAndKind()
	tref := v1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1.CompositionSpec{
			CompositeTypeRef: tref,
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
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
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
					MockList: func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
						compList := &v1.CompositionList{
							Items: []v1.Composition{
								{
									Spec: v1.CompositionSpec{
										CompositeTypeRef: v1.TypeReference{APIVersion: "foreign", Kind: "tome"},
									},
								},
								*comp,
							},
						}
						if list, ok := obj.(*v1.CompositionList); ok {
							compList.DeepCopyInto(list)
							return nil
						}
						t.Errorf("wrong query")
						return nil
					},
				},
				cp: &fake.Composite{
					CompositionSelector: fake.CompositionSelector{Sel: sel},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
					CompositionSelector:   fake.CompositionSelector{Sel: sel},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPILabelSelectorResolver(tc.kube)

			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIDefaultCompositionSelector(t *testing.T) {
	errBoom := errors.New("boom")
	a, k := schema.EmptyObjectKind.GroupVersionKind().ToAPIVersionAndKind()
	tref := v1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: v1.CompositionSpec{
			CompositeTypeRef: tref,
		},
	}

	type args struct {
		kube   client.Client
		defRef corev1.ObjectReference
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
				defRef: corev1.ObjectReference{},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
		},
		"SelectorInPlace": {
			reason: "Should be no-op if a composition selector is in place",
			args: args{
				defRef: corev1.ObjectReference{},
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
			reason: "Should return error if XRD cannot be retrieved",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetXRD),
				cp:  &fake.Composite{},
			},
		},
		"Success": {
			reason: "Successfully set the default composition reference",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						switch cr := obj.(type) {
						case *v2.CompositeResourceDefinition:
							withRef := &v2.CompositeResourceDefinition{Spec: v2.CompositeResourceDefinitionSpec{DefaultCompositionRef: &v2.CompositionReference{Name: comp.Name}}}
							withRef.DeepCopyInto(cr)
							return nil
						case *v1.Composition:
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
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPIDefaultCompositionSelector(tc.kube, tc.defRef, event.NewNopRecorder())

			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
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
	tref := v1.TypeReference{APIVersion: a, Kind: k}
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: v1.CompositionSpec{
			CompositeTypeRef: tref,
		},
	}

	type args struct {
		def v2.CompositeResourceDefinition
		cp  resource.Composite
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
		"NoEnforced": {
			reason: "Should be no-op if no enforced composition ref is given in definition",
			args: args{
				def: v2.CompositeResourceDefinition{},
				cp:  &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{},
			},
		},
		"EnforcedAlreadySet": {
			reason: "Should be no-op if enforced composition reference is already set",
			args: args{
				def: v2.CompositeResourceDefinition{
					Spec: v2.CompositeResourceDefinitionSpec{EnforcedCompositionRef: &v2.CompositionReference{Name: comp.Name}},
				},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
		},
		"Success": {
			reason: "Successfully set the default composition reference",
			args: args{
				def: v2.CompositeResourceDefinition{
					Spec: v2.CompositeResourceDefinitionSpec{EnforcedCompositionRef: &v2.CompositionReference{Name: comp.Name}},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
		},
		"SuccessOverride": {
			reason: "Successfully set the default composition reference even if another one was set",
			args: args{
				def: v2.CompositeResourceDefinition{
					Spec: v2.CompositeResourceDefinitionSpec{EnforcedCompositionRef: &v2.CompositionReference{Name: comp.Name}},
				},
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: "ola"}},
				},
			},
			want: want{
				cp: &fake.Composite{
					CompositionReferencer: fake.CompositionReferencer{Ref: &corev1.ObjectReference{Name: comp.Name}},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewEnforcedCompositionSelector(tc.def, event.NewNopRecorder())

			err := c.SelectComposition(context.Background(), tc.args.cp)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nSelectComposition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPINamingConfigurator(t *testing.T) {
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
		"LabelAlreadyExists": {
			reason: "No operation should be done if the name prefix is already given",
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{xcrd.LabelKeyNamePrefixForComposed: "given"}}},
			},
			want: want{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{xcrd.LabelKeyNamePrefixForComposed: "given"}}},
			},
		},
		"AssignedName": {
			reason: "Its own name should be used as name prefix if it is not given",
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Name: "cp"}},
			},
			want: want{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Name: "cp", Labels: map[string]string{xcrd.LabelKeyNamePrefixForComposed: "cp"}}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPINamingConfigurator(tc.kube)

			err := c.Configure(context.Background(), tc.args.cp, nil)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

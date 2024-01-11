/*
Copyright 2021 The Crossplane Authors.

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

package composition

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	ctrl := true

	compDev := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-composition",
			UID:  types.UID("no-you-uid"),
			Labels: map[string]string{
				"channel": "dev",
			},
		},
	}

	compStaging := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-composition",
			UID:  types.UID("no-you-uid"),
			Labels: map[string]string{
				"channel": "staging",
			},
		},
	}

	compDevWithAnn := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-composition",
			UID:  types.UID("no-you-uid"),
			Labels: map[string]string{
				"channel": "dev",
			},
			Annotations: map[string]string{
				"myannotation": "coolannotation",
			},
		},
	}

	// Not owned by the above composition.
	rev1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: compDev.GetName() + "-1",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "some-other-uid",
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
			Labels: map[string]string{
				v1.LabelCompositionName: compDev.Name,
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 1},
	}

	// Owned by the above composition, but with an 'older' hash.
	rev2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: compDev.GetName() + "-2",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                compDev.GetUID(),
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
			Labels: map[string]string{
				v1.LabelCompositionHash: "some-older-hash",
				v1.LabelCompositionName: compDev.Name,
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 2},
	}

	// Owned by the above composition, with a current hash. The revision number
	// indicates it is the current revision, and thus should not be updated.
	rev3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: compDev.GetName() + "-3",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                compDev.GetUID(),
				Controller:         &ctrl,
				BlockOwnerDeletion: &ctrl,
			}},
			Labels: map[string]string{
				v1.LabelCompositionHash: compDev.Hash()[:63],
				v1.LabelCompositionName: compDev.Name,
				"channel":               "dev",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 3},
	}

	// Should be owned by the above composition, but ownership was stripped out.
	rev4 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: compDev.GetName() + "-4",
			Labels: map[string]string{
				v1.LabelCompositionHash: "some-other-hash",
				v1.LabelCompositionName: compDev.Name,
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 2},
	}

	type args struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}
	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CompositionNotFound": {
			reason: "We should not return an error if the Composition was not found.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetCompositionError": {
			reason: "We should return any other error encountered while getting a Composition.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"CompositionDeleted": {
			reason: "We should return without error if the Composition exists but is being deleted.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							now := metav1.Now()
							*obj.(*v1.Composition) = v1.Composition{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}}
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"ListCompositionRevisionsError": {
			reason: "We should return any error encountered while listing CompositionRevisions.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet:  test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListRevs),
			},
		},
		"SuccessfulNoOp": {
			reason: "We should not create a new CompositionRevision if one exists that matches the Composition's hash.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compDev
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by the above composition with a current hash.
									// This indicates we don't need to create a new revision.
									*rev3,
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"SuccessfulOwnershipUpdate": {
			reason: "We should control existing composition revisions if ownership was stripped out.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compDev
							return nil
						}),
						MockList: func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
							if len(opts) < 1 || opts[0].(client.MatchingLabels)[v1.LabelCompositionName] != compDev.Name {
								t.Errorf("unexpected list options: %v", opts)
							}
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by the above composition, but with an older annotation
									*rev3,

									// Should be controlled by the above composition, but ownership was stripped out.
									*rev4,
								},
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
							if owners := obj.GetOwnerReferences(); len(owners) < 1 || owners[0].UID != compDev.GetUID() {
								t.Errorf("unexpected owner reference: %v, ", obj.GetOwnerReferences()[0])
							}
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"AlreadyControlledByAnotherUID": {
			reason: "We should return an error when a composition revision has matching composition name but another controller ref.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compDev
							return nil
						}),
						MockList: func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
							if len(opts) < 1 || opts[0].(client.MatchingLabels)[v1.LabelCompositionName] != compDev.Name {
								t.Errorf("unexpected list options: %v", opts)
							}
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by other composition
									*rev1,

									// Controlled by the above composition, but with an older annotation
									*rev3,
								},
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
							if owners := obj.GetOwnerReferences(); len(owners) < 1 || owners[0].UID != compDev.GetUID() {
								t.Errorf("unexpected owner reference: %v, ", obj.GetOwnerReferences()[0])
							}
							return nil
						}),
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf("%s is already controlled by   (UID some-other-uid)", rev1.GetName()), errOwnRev),
			},
		},
		"CreateCompositionRevisionError": {
			reason: "We should return any error encountered while creating a CompositionRevision.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockList:   test.NewMockListFn(nil),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateRev),
			},
		},
		"SuccessfulCreation": {
			reason: "We should increase the revision number by one when creating a new CompositionRevision.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compDev
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by the above composition, but with an older hash.
									// This indicates we need to create a new composition.
									*rev2,
								},
							}
							return nil
						}),
						MockCreate: test.NewMockCreateFn(nil, func(got client.Object) error {
							want := NewCompositionRevision(compDev, rev2.Spec.Revision+1)

							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("Create(): -want, +got:\n%s", diff)
							}

							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"SuccessfulCreationLabelUpdate": {
			reason: "We should create a new composition revision when we update labels.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compStaging
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by the above composition with previous label
									*rev3,
								},
							}
							return nil
						}),
						MockCreate: test.NewMockCreateFn(nil, func(got client.Object) error {
							want := NewCompositionRevision(compStaging, rev3.Spec.Revision+1)

							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("Create(): -want, +got:\n%s", diff)
							}

							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"SuccessfulCreationAnnotationUpdate": {
			reason: "We should create a new composition revision when we update annotations.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1.Composition) = *compDevWithAnn
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							*obj.(*v1.CompositionRevisionList) = v1.CompositionRevisionList{
								Items: []v1.CompositionRevision{
									// Controlled by the above composition, but with an older annotation
									*rev3,
								},
							}
							return nil
						}),
						MockCreate: test.NewMockCreateFn(nil, func(got client.Object) error {
							want := NewCompositionRevision(compDevWithAnn, rev3.Spec.Revision+1)

							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("Create(): -want, +got:\n%s", diff)
							}

							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, append(tc.args.opts, WithLogger(testLog))...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

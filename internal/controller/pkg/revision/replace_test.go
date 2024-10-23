/*
Copyright 2024 The Crossplane Authors.

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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestListResolver(t *testing.T) {
	type params struct {
		c  client.Client
		pl v1.PackageList
	}
	type args struct {
		ctx    context.Context
		source string
	}
	type want struct {
		pkg v1.Package
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ListErr": {
			reason: "Errors listing packages should be returned.",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(errors.New("boom")),
				},
				pl: &v1.ConfigurationList{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SourceNotFound": {
			reason: "We should return a NotFound error when no matching package is found.",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*v1.ConfigurationList).Items = []v1.Configuration{
							// This should be skipped due to the un-parsable source.
							{
								Spec: v1.ConfigurationSpec{
									PackageSpec: v1.PackageSpec{
										Package: "I'm invalid!",
									},
								},
							},
							// This should be skipped due to the non-matching source.
							{
								Spec: v1.ConfigurationSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.upbound.io/example/notamatch:v1.0.0",
									},
								},
							},
						}
						return nil
					}),
				},
				pl: &v1.ConfigurationList{},
			},
			args: args{
				source: "xpkg.upbound.io/example/test",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SourceFound": {
			reason: "The package with a matching source should be returned.",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*v1.ConfigurationList).Items = []v1.Configuration{
							{
								Spec: v1.ConfigurationSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.upbound.io/example/test:v1.0.0",
									},
								},
							},
						}
						return nil
					}),
				},
				pl: &v1.ConfigurationList{},
			},
			args: args{
				source: "xpkg.upbound.io/example/test",
			},
			want: want{
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/example/test:v1.0.0",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewListResolver(tc.params.c, tc.params.pl)
			pkg, err := r.Resolve(tc.args.ctx, tc.args.source)

			if diff := cmp.Diff(tc.want.pkg, pkg); diff != "" {
				t.Errorf("%s\nr.Resolve(...): -want pkg, +got pkg:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nr.Resolve(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeactivate(t *testing.T) {
	type params struct {
		c  client.Client
		rl v1.PackageRevisionList
	}
	type args struct {
		ctx context.Context
		pkg v1.Package
	}
	type want struct {
		deactivated bool
		err         error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"UpdatePackageError": {
			reason: "Errors updating the package should be returned.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errors.New("boom")),
				},
			},
			args: args{
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListRevisionsError": {
			reason: "Errors updating the package should be returned.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockList:   test.NewMockListFn(errors.New("boom")),
				},
				rl: &v1.ConfigurationRevisionList{},
			},
			args: args{
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
				},
			},
			want: want{
				// We deactivated the package, but not the revision.
				deactivated: true,
				err:         cmpopts.AnyError,
			},
		},
		"UpdateRevisionError": {
			reason: "Errors updating the package should be returned.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						if _, ok := obj.(*v1.ConfigurationRevision); ok {
							return errors.New("boom")
						}
						return nil
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*v1.ConfigurationRevisionList).Items = []v1.ConfigurationRevision{{
							ObjectMeta: metav1.ObjectMeta{
								OwnerReferences: []metav1.OwnerReference{{
									Controller: ptr.To(true),
									UID:        "pkg",
								}},
							},
							Spec: v1.PackageRevisionSpec{
								DesiredState: v1.PackageRevisionActive,
							},
						}}
						return nil
					}),
				},
				rl: &v1.ConfigurationRevisionList{},
			},
			args: args{
				pkg: &v1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("pkg"),
					},
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
				},
			},
			want: want{
				// We deactivated the package, but not the revision.
				deactivated: true,
				err:         cmpopts.AnyError,
			},
		},
		"DeactivatePackageAndRevision": {
			reason: "We should return true if we deactivate the package and the revision.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*v1.ConfigurationRevisionList).Items = []v1.ConfigurationRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									OwnerReferences: []metav1.OwnerReference{{
										// This owner isn't the controller.
									}},
								},
								Spec: v1.PackageRevisionSpec{
									DesiredState: v1.PackageRevisionActive,
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									OwnerReferences: []metav1.OwnerReference{{
										Controller: ptr.To(true),
										UID:        "not-our-revision",
									}},
								},
								Spec: v1.PackageRevisionSpec{
									DesiredState: v1.PackageRevisionActive,
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									OwnerReferences: []metav1.OwnerReference{{
										Controller: ptr.To(true),
										UID:        "pkg",
									}},
								},
								Spec: v1.PackageRevisionSpec{
									DesiredState: v1.PackageRevisionActive,
								},
							},
						}
						return nil
					}),
				},
				rl: &v1.ConfigurationRevisionList{},
			},
			args: args{
				pkg: &v1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("pkg"),
					},
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
				},
			},
			want: want{
				// We deactivated the package and the revision.
				deactivated: true,
				err:         nil,
			},
		},
		"AlreadyInactive": {
			reason: "We should return false if the package and all of its revisions are already inactive.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*v1.ConfigurationRevisionList).Items = []v1.ConfigurationRevision{{
							ObjectMeta: metav1.ObjectMeta{
								OwnerReferences: []metav1.OwnerReference{{
									Controller: ptr.To(true),
									UID:        "pkg",
								}},
							},
							Spec: v1.PackageRevisionSpec{
								DesiredState: v1.PackageRevisionInactive,
							},
						}}
						return nil
					}),
				},
				rl: &v1.ConfigurationRevisionList{},
			},
			args: args{
				pkg: &v1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("pkg"),
					},
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.ManualActivation,
						},
					},
				},
			},
			want: want{
				deactivated: false,
				err:         nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewPackageAndRevisionDeactivator(tc.params.c, tc.params.rl)
			deactivated, err := d.Deactivate(tc.args.ctx, tc.args.pkg)

			if diff := cmp.Diff(tc.want.deactivated, deactivated); diff != "" {
				t.Errorf("%s\nr.Resolve(...): -want pkg, +got pkg:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nr.Resolve(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

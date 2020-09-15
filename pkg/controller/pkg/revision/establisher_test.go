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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var _ Establisher = &APIEstablisher{}

var trueVal = true

func TestAPIEstablisherCheck(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		est     *APIEstablisher
		objs    []runtime.Object
		parent  resource.Object
		control bool
	}

	type want struct {
		err  error
		objs []currentDesired
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulExistsEstablishControl": {
			reason: "All checks should be successful if we can establish control for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				objs: []currentDesired{{
					Current: &apiextensions.CustomResourceDefinition{},
					Desired: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test", Controller: &trueVal}},
						},
					},
					Exists: true,
				}},
			},
		},
		"SuccessfulNotExistEstablishControl": {
			reason: "All checks should be successful if we can establish control for a parent of new objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				objs: []currentDesired{{
					Desired: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test", Controller: &trueVal}},
						},
					},
					Exists: false,
				}},
			},
		},
		"SuccessfulExistsEstablishOwnership": {
			reason: "All checks should be successful if we can establish ownership for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: false,
			},
			want: want{
				objs: []currentDesired{{
					Current: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test"}},
						},
					},
					Desired: &apiextensions.CustomResourceDefinition{},
					Exists:  true,
				}},
			},
		},
		"SuccessfulNotExistEstablishOwnership": {
			reason: "All checks should be successful if we can establish ownership for a parent of new objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: false,
			},
			want: want{
				objs: []currentDesired{{
					Desired: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test"}},
						},
					},
					Exists: false,
				}},
			},
		},
		"FailedGet": {
			reason: "Cannot determine ability to own or control object if we cannot get it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent:  &v1alpha1.ProviderRevision{},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
		"FailedCreate": {
			reason: "Cannot establish control of object if we cannot create it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
				objs: []currentDesired{{
					Desired: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test", Controller: &trueVal}},
						},
					},
					Exists: false,
				}},
			},
		},
		"FailedUpdate": {
			reason: "Cannot establish control of existing object if we cannot update it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&apiextensions.CustomResourceDefinition{},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
				objs: []currentDesired{{
					Current: &apiextensions.CustomResourceDefinition{},
					Desired: &apiextensions.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{{Name: "test", Controller: &trueVal}},
						},
					},
					Exists: true,
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.est.Check(context.TODO(), tc.args.objs, tc.args.parent, tc.args.control)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.objs, tc.args.est.allObjs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIEstablisherEstablish(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		est     *APIEstablisher
		parent  resource.Object
		control bool
	}

	type want struct {
		err  error
		refs []runtimev1alpha1.TypedReference
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: true,
						},
					},
				},
				parent:  &v1alpha1.ProviderRevision{},
				control: true,
			},
			want: want{
				refs: []runtimev1alpha1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of new objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockCreate: test.NewMockCreateFn(nil),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: false,
						},
					},
				},
				parent:  &v1alpha1.ProviderRevision{},
				control: true,
			},
			want: want{
				refs: []runtimev1alpha1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulExistsEstablishOwnership": {
			reason: "Establishment should be successful if we can establish ownership for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: true,
						},
					},
				},
				parent:  &v1alpha1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []runtimev1alpha1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishOwnership": {
			reason: "Establishment should be successful if we can establish ownership for a parent of new objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockCreate: test.NewMockCreateFn(nil),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: false,
						},
					},
				},
				parent:  &v1alpha1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []runtimev1alpha1.TypedReference{{Name: "ref-me"}},
			},
		},
		"FailedCreate": {
			reason: "Cannot establish control of object if we cannot create it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: false,
						},
					},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
		"FailedUpdate": {
			reason: "Cannot establish control of object if we cannot update it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					allObjs: []currentDesired{
						{
							Current: &apiextensions.CustomResourceDefinition{},
							Desired: &apiextensions.CustomResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ref-me",
								},
							},
							Exists: true,
						},
					},
				},
				parent: &v1alpha1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.est.Establish(context.TODO(), tc.args.parent, tc.args.control)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.refs, tc.args.est.resourceRefs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

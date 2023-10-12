/*
Copyright 2023 The Crossplane Authors.

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

package xrd

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

var _ admission.CustomValidator = &validator{}

func TestValidateUpdate(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		old    runtime.Object
		new    *v1.CompositeResourceDefinition
		client client.Client
	}
	cases := map[string]struct {
		args
		warns admission.Warnings
		err   error
	}{
		"UnexpectedType": {
			args: args{
				old: &extv1.CustomResourceDefinition{},
				new: &v1.CompositeResourceDefinition{},
			},
			err: errors.New(errUnexpectedType),
		},
		"SuccessNoClaimCreate": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil),
				},
			},
		},
		"SuccessWithClaimCreate": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil),
				},
			},
		},

		"SuccessNoClaimUpdate": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
		},
		"SuccessWithClaimUpdate": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
		},
		"FailChangeClaimKind": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "C",
							Plural:   "cs",
							Singular: "c",
							ListKind: "CList",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			// WARN: brittle test, depends on the sorting of the field.ErrorList
			err: field.ErrorList{
				field.Invalid(field.NewPath("spec", "claimNames", "plural"), "cs", "field is immutable"),
				field.Invalid(field.NewPath("spec", "claimNames", "kind"), "C", "field is immutable"),
			}.ToAggregate(),
		},
		"FailOnClaimNotFound": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "B" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
		"FailOnClaimFound": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "B" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
		"FailOnCompositeNotFound": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "A" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
		"FailOnCompositeFound": {
			args: args{
				old: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				new: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "A" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			handler := &validator{
				client: tc.client,
			}
			warns, err := handler.ValidateUpdate(context.TODO(), tc.old, tc.new)
			if diff := cmp.Diff(tc.warns, warns); diff != "" {
				t.Errorf("ValidateUpdate(): -want warnings, +got warnings:\n%s", diff)
			}
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				if d := cmp.Diff(tc.err, err, cmpopts.EquateErrors()); d != "" {
					t.Errorf("ValidateUpdate(): -want error, +got error:\n%s", diff)
				}
			}
		})
	}
}

func TestValidateCreate(t *testing.T) {
	type args struct {
		obj    *v1.CompositeResourceDefinition
		client client.Client
	}
	errBoom := errors.New("boom")
	cases := map[string]struct {
		args
		warns admission.Warnings
		err   error
	}{
		"SuccessNoClaim": {
			args: args{
				obj: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil),
				},
			},
		},
		"SuccessWithClaim": {
			args: args{
				obj: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil),
				},
			},
		},
		"FailOnClaim": {
			args: args{
				obj: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "B" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
		"FailOnComposite": {
			args: args{
				obj: &v1.CompositeResourceDefinition{
					Spec: v1.CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "A",
							Plural:   "as",
							Singular: "a",
							ListKind: "AList",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:     "B",
							Plural:   "bs",
							Singular: "b",
							ListKind: "BList",
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
						p, err := fieldpath.PaveObject(obj)
						if err != nil {
							return err
						}
						s, err := p.GetString("spec.names.kind")
						if err != nil {
							return err
						}
						if s == "A" {
							return errBoom
						}
						return nil
					}),
				},
			},
			err: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			handler := &validator{
				client: tc.client,
			}
			warns, err := handler.ValidateCreate(context.TODO(), tc.obj)
			if diff := cmp.Diff(tc.warns, warns); diff != "" {
				t.Errorf("ValidateUpdate(): -want warnings, +got warnings:\n%s", diff)
			}
			if diff := cmp.Diff(tc.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("ValidateUpdate(): -want error, +got error:\n%s", diff)
			}
		})
	}
}

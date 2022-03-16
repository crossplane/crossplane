/*
Copyright 2022 The Crossplane Authors.

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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestValidateUpdate(t *testing.T) {
	type args struct {
		old runtime.Object
		new *CompositeResourceDefinition
	}
	cases := map[string]struct {
		args
		err error
	}{
		"UnexpectedType": {
			args: args{
				old: &extv1.CustomResourceDefinition{},
				new: &CompositeResourceDefinition{},
			},
			err: errors.New(errUnexpectedType),
		},
		"GroupChanged": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Group: "a",
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Group: "b",
					},
				},
			},
			err: errors.New(errGroupImmutable),
		},
		"PluralChanged": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "b",
						},
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "a",
						},
					},
				},
			},
			err: errors.New(errPluralImmutable),
		},
		"KindChanged": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "b",
						},
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
			},
			err: errors.New(errKindImmutable),
		},
		"ClaimPluralChanged": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Plural: "b",
						},
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Plural: "a",
						},
					},
				},
			},
			err: errors.New(errClaimPluralImmutable),
		},
		"ClaimKindChanged": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind: "b",
						},
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
			},
			err: errors.New(errClaimKindImmutable),
		},
		"Success": {
			args: args{
				old: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
				new: &CompositeResourceDefinition{
					Spec: CompositeResourceDefinitionSpec{
						Names: extv1.CustomResourceDefinitionNames{
							Kind: "a",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.new.ValidateUpdate(tc.old)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ValidateUpdate(): -want, +got:\n%s", diff)
			}
		})
	}
}

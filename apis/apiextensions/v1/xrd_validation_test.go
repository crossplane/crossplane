// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestValidateConversion(t *testing.T) {
	cases := map[string]struct {
		reason string
		c      *CompositeResourceDefinition
		want   field.ErrorList
	}{
		"Valid": {
			reason: "A CompositeResourceDefinition with a valid conversion should be accepted",
			c: &CompositeResourceDefinition{
				Spec: CompositeResourceDefinitionSpec{
					Conversion: &extv1.CustomResourceConversion{
						Strategy: extv1.NoneConverter,
					},
				},
			},
		},
		"ValidWebhook": {
			reason: "A CompositeResourceDefinition with a valid webhook conversion should be accepted",
			c: &CompositeResourceDefinition{
				Spec: CompositeResourceDefinitionSpec{
					Conversion: &extv1.CustomResourceConversion{
						Strategy: extv1.WebhookConverter,
						Webhook: &extv1.WebhookConversion{
							ClientConfig: &extv1.WebhookClientConfig{},
						},
					},
				},
			},
		},
		"InvalidWebhook": {
			reason: "A CompositeResourceDefinition with an invalid webhook conversion should be rejected",
			c: &CompositeResourceDefinition{
				Spec: CompositeResourceDefinitionSpec{
					Conversion: &extv1.CustomResourceConversion{
						Strategy: extv1.WebhookConverter,
					},
				},
			},
			want: field.ErrorList{
				field.Required(field.NewPath("spec", "conversion", "webhook"), ""),
			},
		},
	}
	for tcName, tc := range cases {
		t.Run(tcName, func(t *testing.T) {
			got := tc.c.validateConversion()
			if diff := cmp.Diff(tc.want, got, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("\n%s\nValidateConversion(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	type args struct {
		old *CompositeResourceDefinition
		new *CompositeResourceDefinition
	}
	cases := map[string]struct {
		args
		warns admission.Warnings
		errs  field.ErrorList
	}{
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
			errs: field.ErrorList{field.Invalid(field.NewPath("spec", "group"), "b", "")},
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
			errs: field.ErrorList{field.Invalid(field.NewPath("spec", "names", "plural"), "a", "")},
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
			errs: field.ErrorList{field.Invalid(field.NewPath("spec", "names", "kind"), "a", "")},
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
			errs: field.ErrorList{field.Invalid(field.NewPath("spec", "claimNames", "plural"), "a", "")},
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
			errs: field.ErrorList{field.Invalid(field.NewPath("spec", "claimNames", "kind"), "a", "")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, gotErr := tc.new.ValidateUpdate(tc.old)
			if diff := cmp.Diff(tc.errs, gotErr, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("\nValidateUpdate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

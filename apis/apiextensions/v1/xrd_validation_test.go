package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

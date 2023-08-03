package v1

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks that the supplied CompositeResourceDefinition spec is logically valid.
func (c *CompositeResourceDefinition) Validate() (warns []string, errs field.ErrorList) {
	type validationFunc func() field.ErrorList
	validations := []validationFunc{
		c.validateConversion,
	}
	for _, f := range validations {
		errs = append(errs, f()...)
	}
	return nil, errs
}

// validateConversion checks that the supplied CompositeResourceDefinition spec
func (c *CompositeResourceDefinition) validateConversion() (errs field.ErrorList) {
	if conv := c.Spec.Conversion; conv != nil && conv.Strategy == extv1.WebhookConverter &&
		(conv.Webhook == nil || conv.Webhook.ClientConfig == nil) {
		errs = append(errs, field.Required(field.NewPath("spec", "conversion", "webhook"), fmt.Sprintf("webhook configuration is required when conversion strategy is %q", extv1.WebhookConverter)))
	}
	return errs
}

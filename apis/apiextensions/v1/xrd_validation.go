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
// is valid w.r.t. conversion.
func (c *CompositeResourceDefinition) validateConversion() (errs field.ErrorList) {
	if conv := c.Spec.Conversion; conv != nil && conv.Strategy == extv1.WebhookConverter &&
		(conv.Webhook == nil || conv.Webhook.ClientConfig == nil) {
		errs = append(errs, field.Required(field.NewPath("spec", "conversion", "webhook"), fmt.Sprintf("webhook configuration is required when conversion strategy is %q", extv1.WebhookConverter)))
	}
	return errs
}

// ValidateUpdate checks that the supplied CompositeResourceDefinition update is valid w.r.t. the old one.
func (c *CompositeResourceDefinition) ValidateUpdate(old *CompositeResourceDefinition) (warns []string, errs field.ErrorList) {
	// Validate the update
	if c.Spec.Group != old.Spec.Group {
		errs = append(errs, field.Invalid(field.NewPath("spec", "group"), c.Spec.Group, "field is immutable"))
	}
	if c.Spec.Names.Plural != old.Spec.Names.Plural {
		errs = append(errs, field.Invalid(field.NewPath("spec", "names", "plural"), c.Spec.Names.Plural, "field is immutable"))
	}
	if c.Spec.Names.Kind != old.Spec.Names.Kind {
		errs = append(errs, field.Invalid(field.NewPath("spec", "names", "kind"), c.Spec.Names.Kind, "field is immutable"))
	}
	if c.Spec.ClaimNames != nil && old.Spec.ClaimNames != nil {
		if c.Spec.ClaimNames.Plural != old.Spec.ClaimNames.Plural {
			errs = append(errs, field.Invalid(field.NewPath("spec", "claimNames", "plural"), c.Spec.ClaimNames.Plural, "field is immutable"))
		}
		if c.Spec.ClaimNames.Kind != old.Spec.ClaimNames.Kind {
			errs = append(errs, field.Invalid(field.NewPath("spec", "claimNames", "kind"), c.Spec.ClaimNames.Kind, "field is immutable"))
		}
	}
	warns, newErr := c.Validate()
	errs = append(errs, newErr...)
	return warns, errs
}

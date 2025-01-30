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

package v1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate performs logical validation of a Composition.
func (c *Composition) Validate() (warns []string, errs field.ErrorList) {
	type validationFunc func() field.ErrorList
	validations := []validationFunc{
		c.validateMode,
		c.validatePipeline,
	}
	for _, f := range validations {
		errs = append(errs, f()...)
	}
	return nil, errs
}

func (c *Composition) validateMode() (errs field.ErrorList) {
	// Pipeline is the only supported mode.
	if len(c.Spec.Pipeline) == 0 {
		errs = append(errs, field.Required(field.NewPath("spec", "pipeline"), "an array of pipeline steps is required in Pipeline mode"))
	}

	return errs
}

func (c *Composition) validatePipeline() (errs field.ErrorList) {
	seen := map[string]bool{}
	for i, f := range c.Spec.Pipeline {
		if seen[f.Step] {
			errs = append(errs, field.Duplicate(field.NewPath("spec", "pipeline").Index(i).Child("step"), f.Step))
		}
		seen[f.Step] = true

		seenCred := map[string]bool{}
		for j, cs := range f.Credentials {
			if seenCred[cs.Name] {
				errs = append(errs, field.Duplicate(field.NewPath("spec", "pipeline").Index(i).Child("credentials").Index(j).Child("name"), cs.Name))
			}
			seenCred[cs.Name] = true

			switch cs.Source {
			case FunctionCredentialsSourceSecret:
				if cs.SecretRef == nil {
					errs = append(errs, field.Required(field.NewPath("spec", "pipeline").Index(i).Child("credentials").Index(j).Child("secretRef"), "must be specified when source is Secret"))
				}
			case FunctionCredentialsSourceNone:
				// No requirements here.
			}
		}
	}
	return errs
}

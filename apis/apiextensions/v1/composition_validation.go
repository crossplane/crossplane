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

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	verrors "github.com/crossplane/crossplane/internal/validation/errors"
)

// Validate performs logical validation of a Composition.
func (c *Composition) Validate() (warns []string, errs field.ErrorList) {
	type validationFunc func() field.ErrorList
	validations := []validationFunc{
		c.validateMode,
		c.validatePatchSets,
		c.validateResources,
		c.validatePipeline,
		c.validateEnvironment,
	}
	for _, f := range validations {
		errs = append(errs, f()...)
	}
	return nil, errs
}

func (c *Composition) validateMode() (errs field.ErrorList) {
	// "Resources" mode was the original mode. It predates the mode field, so
	// it's the default if mode isn't specified.
	m := CompositionModeResources
	if c.Spec.Mode != nil {
		m = *c.Spec.Mode
	}

	switch m {
	case CompositionModeResources:
		if len(c.Spec.Resources) == 0 {
			errs = append(errs, field.Required(field.NewPath("spec", "resources"), "an array of resources is required in Resources mode (the default if no mode is specified)"))
		}
	case CompositionModePipeline:
		if len(c.Spec.Pipeline) == 0 {
			errs = append(errs, field.Required(field.NewPath("spec", "pipeline"), "an array of pipeline steps is required in Pipeline mode"))
		}
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

// validatePatchSets checks that:
// - patchSets are composed of valid patches
// - there are no nested patchSets
// - only existing patchSets are used by resources.
func (c *Composition) validatePatchSets() (errs field.ErrorList) {
	definedPatchSets := make(map[string]bool, len(c.Spec.PatchSets))
	for i, s := range c.Spec.PatchSets {
		definedPatchSets[s.Name] = true
		for j, p := range s.Patches {
			if p.Type == PatchTypePatchSet {
				errs = append(errs, field.Invalid(field.NewPath("spec", "patchSets").Index(i).Child("patches").Index(j).Child("type"), p.Type, errors.New("cannot use patches within patches").Error()))
				continue
			}
			if err := p.Validate(); err != nil {
				errs = append(errs, verrors.WrapFieldError(err, field.NewPath("spec", "patchSets").Index(i).Child("patches").Index(j)))
			}
		}
	}
	for i, r := range c.Spec.Resources {
		for j, p := range r.Patches {
			if p.Type != PatchTypePatchSet {
				continue
			}
			if p.PatchSetName == nil {
				// already covered by patch c.validateResources, but we don't assume any ordering
				errs = append(errs, field.Required(field.NewPath("spec", "resources").Index(i).Child("patches").Index(j).Child("patchSetName"), "must be specified when type is patchSet"))
				continue
			}
			if !definedPatchSets[*p.PatchSetName] {
				errs = append(errs, field.Invalid(field.NewPath("spec", "resources").Index(i).Child("patches").Index(j).Child("patchSetName"), p.PatchSetName, "patchSetName must be the name of a declared patchSet"))
			}
		}
	}
	return errs
}

func (c *Composition) validateResources() (errs field.ErrorList) {
	if err := c.validateResourceNames(); err != nil {
		errs = append(errs, err...)
	}
	for i, res := range c.Spec.Resources {
		for j, patch := range res.Patches {
			if err := patch.Validate(); err != nil {
				errs = append(errs, verrors.WrapFieldError(err, field.NewPath("spec", "resources").Index(i).Child("patches").Index(j)))
			}
		}
		for j, rd := range res.ReadinessChecks {
			if err := rd.Validate(); err != nil {
				errs = append(errs, verrors.WrapFieldError(err, field.NewPath("spec", "resources").Index(i).Child("readinessChecks").Index(j)))
			}
		}
		// TODO(phisco): we should validate also ConnectionDetails, but would need a major refactoring
	}
	return errs
}

// validateResourceNames checks that:
//  1. Either all resources have a name or they are all anonymous: because if some but not all templates are named it's
//     safest to refuse to operate. We don't have enough information to use the named composer, but using the anonymous
//     composer may be surprising. There's a risk that someone added a new anonymous template to a Composition that
//     otherwise uses named templates. If they added the new template to the beginning or middle of the resources array
//     using the anonymous composer would be destructive, because it assumes template N always corresponds to existing
//     template N.
//  2. All resources have unique names: because other parts of the code require so.
//  3. If the composition has any functions, it must have only named resources: This is necessary for the
//     FunctionComposer to be able to associate entries in the spec.resources array with entries in a RunFunctionRequest's observed
//     and desired objects.
func (c *Composition) validateResourceNames() (errs field.ErrorList) {
	seen := map[string]bool{}
	for resourceIndex, res := range c.Spec.Resources {
		// Check that all resources have a name and that it is unique.
		// If the composition has any functions, it must have only named resources.
		name := res.GetName()
		if name == "" {
			// If the composition has any functions, it must have only named resources.
			if len(c.Spec.Pipeline) != 0 {
				errs = append(errs, field.Required(field.NewPath("spec", "resources").Index(resourceIndex).Child("name"), "cannot have anonymous resources when composition has functions"))
				continue
			}
			// If it's not the first resource, and all previous one were named, then this is an error.
			if resourceIndex != 0 && len(seen) != 0 {
				errs = append(errs, field.Required(field.NewPath("spec", "resources").Index(resourceIndex).Child("name"), "cannot mix named and anonymous resources, all resources must have a name or none must have a name"))
				continue
			}
			continue
		}
		// Check that the name is unique
		if seen[name] {
			errs = append(errs, field.Duplicate(field.NewPath("spec", "resources").Index(resourceIndex).Child("name"), name))
			continue
		}
		// If it's not the first resource, and all previous one were anonymous, then this is an error.
		if resourceIndex != 0 && len(seen) == 0 {
			errs = append(errs, field.Invalid(field.NewPath("spec", "resources").Index(resourceIndex).Child("name"), name, "cannot mix named and anonymous resources, all resources must have a name or none must have a name"))
			continue
		}
		seen[name] = true
	}
	return errs
}

// validateEnvironment checks that the environment is logically valid.
func (c *Composition) validateEnvironment() field.ErrorList {
	if c.Spec.Environment == nil {
		return nil
	}
	if errs := verrors.WrapFieldErrorList(c.Spec.Environment.Validate(), field.NewPath("spec", "environment")); len(errs) > 0 {
		return errs
	}
	return nil
}

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
		c.validatePatchSets,
		c.validateResources,
		c.validateFunctions,
	}
	for _, f := range validations {
		errs = append(errs, f()...)
	}
	return nil, errs
}

func (c *Composition) validateFunctions() (errs field.ErrorList) {
	seen := map[string]bool{}
	for i, f := range c.Spec.Functions {
		if seen[f.Name] {
			errs = append(errs, field.Duplicate(field.NewPath("spec", "functions").Index(i).Child("name"), f.Name))
		}
		seen[f.Name] = true
		if err := f.Validate(); err != nil {
			errs = append(errs, verrors.WrapFieldError(err, field.NewPath("spec", "functions").Index(i)))
		}
	}
	return errs
}

func (c *Composition) validatePatchSets() (errs field.ErrorList) {
	for i, s := range c.Spec.PatchSets {
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
//     FunctionComposer to be able to associate entries in the spec.resources array with entries in a FunctionIO's observed
//     and desired arrays
func (c *Composition) validateResourceNames() (errs field.ErrorList) {
	seen := map[string]bool{}
	for resourceIndex, res := range c.Spec.Resources {
		// Check that all resources have a name and that it is unique.
		// If the composition has any functions, it must have only named resources.
		name := res.GetName()
		if name == "" {
			// If the composition has any functions, it must have only named resources.
			if len(c.Spec.Functions) != 0 {
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

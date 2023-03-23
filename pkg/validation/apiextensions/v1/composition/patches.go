/*
Copyright 2023 the Crossplane Authors.

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

package composition

import (
	"context"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	xperrors "github.com/crossplane/crossplane/pkg/validation/errors"
	xpschema "github.com/crossplane/crossplane/pkg/validation/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// validatePatchesWithSchemas validates the patches of a composition against the resources schemas.
func (v *Validator) validatePatchesWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	// Let's first dereference patchSets
	resources, err := composite.ComposedTemplates(comp.Spec)
	if err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "resources"), comp.Spec.Resources, err.Error()))
		return errs
	}
	for i, resource := range resources {
		for j := range resource.Patches {
			if err := v.validatePatchWithSchemas(ctx, comp, i, j); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// validatePatchWithSchemas validates a patch against the resources schemas.
func (v *Validator) validatePatchWithSchemas( //nolint:gocyclo // TODO(phisco): refactor
	ctx context.Context,
	comp *v1.Composition,
	resourceNumber, patchNumber int,
) *field.Error {
	if len(comp.Spec.Resources) <= resourceNumber {
		return field.InternalError(field.NewPath("spec", "resources").Index(resourceNumber), errors.Errorf("cannot find resource"))
	}
	if len(comp.Spec.Resources[resourceNumber].Patches) <= patchNumber {
		return field.InternalError(field.NewPath("spec", "resources").Index(resourceNumber).Child("patches").Index(patchNumber), errors.Errorf("cannot find patch"))
	}
	resource := comp.Spec.Resources[resourceNumber]
	patch := resource.Patches[patchNumber]
	res, err := resource.GetBaseObject()
	if err != nil {
		return field.Invalid(field.NewPath("spec", "resources").Index(resourceNumber).Child("base"), resource.Base, err.Error())
	}

	compositeCRD, err := v.crdGetter.Get(ctx, schema.FromAPIVersionAndKind(
		comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind,
	))
	if err != nil {
		return field.InternalError(field.NewPath("spec"), errors.Wrapf(err, "cannot find composite type %s: %s", comp.Spec.CompositeTypeRef, err))
	}
	resourceCRD, err := v.crdGetter.Get(ctx, res.GetObjectKind().GroupVersionKind())
	if err != nil {
		return field.InternalError(field.NewPath("spec"), errors.Errorf("cannot find resource type %s: %s", res.GetObjectKind().GroupVersionKind(), err))
	}

	var validationErr *field.Error
	var fromType, toType xpschema.KnownJSONType
	switch patch.GetType() {
	case v1.PatchTypeFromCompositeFieldPath:
		fromType, toType, validationErr = ValidateFromCompositeFieldPathPatch(
			patch,
			compositeCRD.Spec.Validation.OpenAPIV3Schema,
			resourceCRD.Spec.Validation.OpenAPIV3Schema,
		)
	case v1.PatchTypeToCompositeFieldPath:
		fromType, toType, validationErr = ValidateFromCompositeFieldPathPatch(
			patch,
			resourceCRD.Spec.Validation.OpenAPIV3Schema,
			compositeCRD.Spec.Validation.OpenAPIV3Schema,
		)
	case v1.PatchTypeCombineFromComposite:
		fromType, toType, validationErr = ValidateCombineFromCompositePathPatch(
			patch,
			compositeCRD.Spec.Validation.OpenAPIV3Schema,
			resourceCRD.Spec.Validation.OpenAPIV3Schema)
	case v1.PatchTypeCombineToComposite:
		fromType, toType, validationErr = ValidateCombineFromCompositePathPatch(
			patch,
			resourceCRD.Spec.Validation.OpenAPIV3Schema,
			compositeCRD.Spec.Validation.OpenAPIV3Schema)
	case v1.PatchTypePatchSet:
		// Should never happen as patchSets should have been dereferenced and removed by now.
		return field.InternalError(field.NewPath("spec", "resources").Index(resourceNumber).Child("patches").Index(patchNumber), errors.Errorf("patchSet should have been dereferenced"))
	case v1.PatchTypeFromEnvironmentFieldPath,
		v1.PatchTypeCombineFromEnvironment, v1.PatchTypeToEnvironmentFieldPath,
		v1.PatchTypeCombineToEnvironment:
		// TODO(phisco): implement validation for environment related patches
		return nil
	}
	if validationErr != nil {
		return xperrors.WrapFieldError(validationErr, field.NewPath("spec", "resources").Index(resourceNumber).Child("patches").Index(patchNumber))
	}

	return xperrors.WrapFieldError(
		validateIOTypesWithTransforms(patch.Transforms, fromType, toType),
		field.NewPath("spec", "resources").Index(resourceNumber).Child("patches").Index(patchNumber),
	)
}

// ValidateCombineFromCompositePathPatch validates Combine Patch types, by going through and validating the fromField
// path variables, checking if they all need to be required, checking if the right combine strategy is set and
// validating transforms.
//
//nolint:gocyclo // TODO(phico): there is not much we can refactor here
func ValidateCombineFromCompositePathPatch(
	patch v1.Patch,
	from *apiextensions.JSONSchemaProps,
	to *apiextensions.JSONSchemaProps,
) (fromType, toType xpschema.KnownJSONType, err *field.Error) {
	toFieldPath := patch.GetToFieldPath()
	toType, toRequired, toFieldPathErr := validateFieldPath(to, toFieldPath)
	if toFieldPathErr != nil {
		return "", "", field.Invalid(field.NewPath("toFieldPath"), toFieldPath, toFieldPathErr.Error())
	}
	errs := field.ErrorList{}
	for _, variable := range patch.Combine.Variables {
		fromFieldPath := variable.FromFieldPath
		_, fromRequired, err := validateFieldPath(from, fromFieldPath)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("fromFieldPath"), fromFieldPath, err.Error()))
			continue
		}

		if patch.Policy.GetFromFieldPathPolicy() == v1.FromFieldPathPolicyRequired && !fromRequired {
			return "", "", field.Invalid(
				field.NewPath("policy", "fromFieldPath"),
				patch.Policy.FromFieldPath,
				"fromFieldPath is required according to the schema, but policy is set to optional",
			)
		}
		if toRequired && !fromRequired {
			errs = append(errs, field.Invalid(
				field.NewPath("combine"),
				patch.Combine.Variables,
				fmt.Sprintf("fromFieldPath (%v) is not fromRequired but toFieldPath (%s) is, this could lead to unexpected runtime errors", patch.Combine.Variables, toFieldPath),
			))
			continue
		}
	}

	if len(errs) > 0 {
		return "", "", field.Invalid(field.NewPath("combine"), patch.Combine.Variables, errs.ToAggregate().Error())
	}

	switch patch.Combine.Strategy {
	case v1.CombineStrategyString:
		if patch.Combine.String == nil {
			return "", "", field.Required(field.NewPath("combine", "string"), "string combine strategy requires configuration")
		}
		fromType = xpschema.KnownJSONTypeString
	default:
		return "", "", field.Invalid(field.NewPath("combine", "strategy"), patch.Combine.Strategy, "combine strategy is not supported")
	}

	// TODO(lsviben): check if we could validate the patch combine format

	return fromType, toType, nil
}

// ValidateFromCompositeFieldPathPatch validates a patch of type FromCompositeFieldPath.
func ValidateFromCompositeFieldPathPatch(
	patch v1.Patch,
	from, to *apiextensions.JSONSchemaProps,
) (fromType, toType xpschema.KnownJSONType, res *field.Error) {
	fromFieldPath := patch.GetFromFieldPath()
	toFieldPath := patch.GetToFieldPath()
	fromType, fromRequired, err := validateFieldPath(from, fromFieldPath)
	if err != nil {
		return "", "", field.Invalid(field.NewPath("fromFieldPath"), fromFieldPath, err.Error())
	}

	if patch.Policy.GetFromFieldPathPolicy() == v1.FromFieldPathPolicyRequired && !fromRequired {
		return "", "", field.Invalid(
			field.NewPath("policy", "fromFieldPath"),
			patch.Policy.FromFieldPath,
			"fromFieldPath policy is set to Required, but it is not required according to the source's schema, this could lead to runtime errors",
		)
	}

	toType, toRequired, err := validateFieldPath(to, toFieldPath)
	if err != nil {
		return "", "", field.Invalid(field.NewPath("toFieldPath"), toFieldPath, err.Error())
	}
	if toRequired && !fromRequired {
		return "", "", field.Invalid(field.NewPath("fromFieldPath"), fromFieldPath, fmt.Sprintf(
			"fromFieldPath is optional by the source's schema, but toFieldPath '%s' is required by the destination's schemas, this could lead to runtime errors",
			toFieldPath,
		))
	}

	return fromType, toType, nil
}

//nolint:gocyclo // TODO(phico): there is not much we can refactor here
func validateIOTypesWithTransforms(transforms []v1.Transform, fromType, toType xpschema.KnownJSONType) *field.Error {
	transformedToType, err := v1.FromKnownJSONType(fromType)
	if err != nil && fromType != "" {
		return field.InternalError(field.NewPath("transforms"), err)
	}
	for i, transform := range transforms {
		err := transform.IsValidInput(transformedToType)
		if err != nil && transformedToType != "" {
			return field.Invalid(field.NewPath("transforms").Index(i), transforms, err.Error())
		}
		out, err := transform.GetOutputType()
		if err != nil {
			return field.InternalError(field.NewPath("transforms").Index(i), err)
		}
		if out == nil {
			// no need to validate the rest of the transforms as a nil output without error means we don't
			// have a way to know the output type for some transforms
			return nil
		}
		transformedToType = *out
	}
	if transformedToType == "" || toType == "" {
		return nil
	}
	transformedToJSONType := transformedToType.ToKnownJSONType()

	// integer is a subset of number per JSON specification:
	// https://datatracker.ietf.org/doc/html/draft-zyp-json-schema-04#section-3.5
	if !transformedToJSONType.IsEquivalent(toType) {
		if len(transforms) == 0 {
			return field.Required(field.NewPath("transforms"), fmt.Sprintf("the fromFieldPath does not have a type compatible with the fromFieldPath according to their schemas and no transforms were provided: %s != %s", transformedToType, toType))
		}
		return field.Invalid(field.NewPath("transforms"), transforms, fmt.Sprintf("the provided transforms do not output a type compatible with the toFieldPath according to the schema: %s != %s", transformedToType, toType))
	}

	return nil
}

// validateFieldPath validates the given fieldPath is valid for the given schema.
// It returns the type of the fieldPath and whether it is required, or any error.
// If the returned type is "", but without error, it means the fieldPath is accepted by the schema, but not defined in it.
func validateFieldPath(schema *apiextensions.JSONSchemaProps, fieldPath string) (fieldType xpschema.KnownJSONType, required bool, err error) {
	if fieldPath == "" {
		return "", false, nil
	}
	segments, err := fieldpath.Parse(fieldPath)
	if err != nil {
		return "", false, err
	}
	if len(segments) > 0 && segments[0].Type == fieldpath.SegmentField && segments[0].Field == "metadata" &&
		isMissingMetadataSchema(schema) {
		segments = segments[1:]
		schema = &metadataSchema
	}
	if len(segments) > 0 {
		required = true
	}
	return validateFieldPathSegments(segments, schema, required, fieldPath)
}

func validateFieldPathSegments(segments fieldpath.Segments, schema *apiextensions.JSONSchemaProps, required bool, fieldPath string) (xpschema.KnownJSONType, bool, error) {
	current := schema
	for _, segment := range segments {
		currentSegment, segmentRequired, err := validateFieldPathSegment(current, segment)
		if err != nil {
			return "", false, err
		}
		if currentSegment == nil {
			return "", false, nil
		}
		current = currentSegment
		required = required && segmentRequired
	}

	if !xpschema.IsKnownJSONType(current.Type) {
		return "", false, fmt.Errorf("field path %q has an unsupported type %q", fieldPath, current.Type)
	}
	return xpschema.KnownJSONType(current.Type), required, nil
}

func isMissingMetadataSchema(schema *apiextensions.JSONSchemaProps) bool {
	if schema == nil || schema.Properties == nil {
		return true
	}
	m, defined := schema.Properties["metadata"]
	if !defined || m.Properties == nil || len(m.Properties) == 0 {
		return true
	}
	return false
}

// validateFieldPathSegment validates that the given field path segment is valid for the given schema.
// It returns the schema for the segment, whether the segment is required, and an error if the segment is invalid.
// It returns the schema for the segment, whether the segment is wantRequired, and an error if the segment is invalid.
func validateFieldPathSegment(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (current *apiextensions.JSONSchemaProps, required bool, err error) {
	switch segment.Type {
	case fieldpath.SegmentField:
		return validateFieldPathSegmentField(parent, segment)
	case fieldpath.SegmentIndex:
		return validateFieldPathSegmentIndex(parent, segment)
	}
	return nil, false, nil
}

func validateFieldPathSegmentField(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (*apiextensions.JSONSchemaProps, bool, error) {
	if parent == nil {
		return nil, false, nil
	}
	if segment.Type != fieldpath.SegmentField {
		return nil, false, errors.Errorf("segment is not a field")
	}
	if propType := parent.Type; propType != "" && propType != string(xpschema.KnownJSONTypeObject) {
		return nil, false, errors.Errorf("trying to access a field '%s' of object, but schema says parent is of type: '%v'", segment.Field, propType)
	}
	// TODO(phisco): handle other fields, e.g. CEL?
	prop, exists := parent.Properties[segment.Field]
	if !exists {
		if pointer.BoolDeref(parent.XPreserveUnknownFields, false) {
			return nil, false, nil
		}
		if parent.AdditionalProperties != nil && parent.AdditionalProperties.Allows {
			return parent.AdditionalProperties.Schema, false, nil
		}
		return nil, false, errors.Errorf("field '%s' is not valid according to the schema", segment.Field)
	}
	return &prop, slices.Contains(parent.Required, segment.Field), nil
}

func validateFieldPathSegmentIndex(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (*apiextensions.JSONSchemaProps, bool, error) {
	if parent == nil {
		return nil, false, nil
	}
	if segment.Type != fieldpath.SegmentIndex {
		return nil, false, errors.Errorf("segment is not an index")
	}
	if parent.Type != string(xpschema.KnownJSONTypeArray) {
		return nil, false, errors.Errorf("trying to access a '%s' by index", parent.Type)
	}
	if parent.Items == nil {
		return nil, false, errors.New("no items found in array")
	}
	// if there is a limit on max items and the index is above that, return an error
	if parent.MaxItems != nil && *parent.MaxItems < int64(segment.Index+1) {
		return nil, false, errors.Errorf("index is above the allowed size of the array: %d > %d", segment.Index, *parent.MaxItems)
	}
	if s := parent.Items.Schema; s != nil {
		// return wantRequired if the array has a schema and a minimum size
		return s, parent.MinItems != nil && *parent.MinItems > 0, nil
	}
	schemas := parent.Items.JSONSchemas
	if len(schemas) < int(segment.Index) {
		return nil, false, errors.Errorf("no schemas ")
	}

	// means there is no schema at all for this array
	return nil, false, nil
}

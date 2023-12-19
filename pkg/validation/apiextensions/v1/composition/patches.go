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
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	verrors "github.com/crossplane/crossplane/internal/validation/errors"
	xpschema "github.com/crossplane/crossplane/pkg/validation/internal/schema"
)

const (
	errFmtArrayIndexAboveMax   = "index is above the allowed size of the array: %d > %d"
	errFmtFieldInvalid         = "field '%s' is not valid according to the schema"
	errFmtIndexAccessWrongType = "trying to access a '%s' by index"
	errFmtFieldAccessWrongType = "trying to access a field '%s' of object, but schema says parent is of type: '%v'"
	errUnableToParse           = "cannot parse base"
)

// validatePatchesWithSchemas validates the patches of a composition against the resources schemas.
func (v *Validator) validatePatchesWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	// Let's first dereference patchSets
	for i, resource := range comp.Spec.Resources {
		for j := range resource.Patches {
			if err := v.validatePatchWithSchemas(ctx, comp, i, j); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (v *Validator) validateEnvironmentPatchesWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	if comp.Spec.Environment == nil {
		return nil
	}
	compositeResGVK := schema.FromAPIVersionAndKind(
		comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind,
	)

	compositeCRD, err := v.crdGetter.Get(ctx, compositeResGVK.GroupKind())
	if err != nil {
		return append(errs, field.InternalError(field.NewPath("spec").Child("environment"), errors.Errorf("cannot find composite type %s: %w", comp.Spec.CompositeTypeRef, err)))
	}
	// TODO(phisco): we could relax this condition and handle partially missing crds in the future
	if compositeCRD == nil {
		// means the crdGetter didn't find the needed crds, but didn't return an error
		// this means we should not treat it as an error either
		return nil
	}
	for i, patch := range comp.Spec.Environment.Patches {
		v1Patch := patch.ToPatch()
		if v1Patch == nil {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("environment", "patches").Index(i), patch, "cannot convert patch to v1.Patch"))
			continue
		}
		if err := verrors.WrapFieldError(v.validatePatchWithSchemaInternal(patchValidationCtx{
			comp:            comp,
			patch:           *v1Patch,
			compositeCRD:    compositeCRD,
			compositeResGVK: compositeResGVK,
		}), field.NewPath("spec").Child("environment", "patches").Index(i)); err != nil {
			errs = append(errs, err)

		}
	}
	return errs
}

func getSchemaForVersion(crd *apiextensions.CustomResourceDefinition, version string) *apiextensions.JSONSchemaProps {
	if crd == nil {
		return nil
	}
	if crd.Spec.Validation != nil {
		return crd.Spec.Validation.OpenAPIV3Schema
	}
	for _, v := range crd.Spec.Versions {
		if v.Name == version {
			if v.Schema == nil {
				return nil
			}
			return v.Schema.OpenAPIV3Schema
		}
	}
	return nil
}

// validatePatchWithSchemas validates a patch against the resources schemas.
func (v *Validator) validatePatchWithSchemas(ctx context.Context, comp *v1.Composition, resourceNumber, patchNumber int) *field.Error {
	if len(comp.Spec.Resources) <= resourceNumber {
		return field.InternalError(field.NewPath("spec", "resources").Index(resourceNumber), errors.Errorf("cannot find resource"))
	}
	if len(comp.Spec.Resources[resourceNumber].Patches) <= patchNumber {
		return field.InternalError(field.NewPath("spec", "resources").Index(resourceNumber).Child("patches").Index(patchNumber), errors.Errorf("cannot find patch"))
	}
	resource := comp.Spec.Resources[resourceNumber]
	patch := resource.Patches[patchNumber]
	resourceGVK, err := GetBaseObjectGVK(&resource)
	if err != nil {
		return field.Invalid(field.NewPath("spec", "resources").Index(resourceNumber).Child("base"), resource.Base, err.Error())
	}

	compositeResGVK := schema.FromAPIVersionAndKind(
		comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind,
	)

	compositeCRD, err := v.crdGetter.Get(ctx, compositeResGVK.GroupKind())
	if err != nil {
		return field.InternalError(field.NewPath("spec").Child("resources").Index(resourceNumber), errors.Errorf("cannot find composite type %s: %w", comp.Spec.CompositeTypeRef, err))
	}
	resourceCRD, err := v.crdGetter.Get(ctx, resourceGVK.GroupKind())
	if err != nil {
		return field.InternalError(field.NewPath("spec").Child("resources").Index(resourceNumber), errors.Errorf("cannot find resource type %s: %s", resourceGVK, err))
	}

	// TODO(phisco): we could relax this condition and handle partially missing crds in the future
	if compositeCRD == nil || resourceCRD == nil {
		// means the crdGetter didn't find the needed crds, but didn't return an error
		// this means we should not treat it as an error either
		return nil
	}

	return verrors.WrapFieldError(v.validatePatchWithSchemaInternal(patchValidationCtx{
		comp:            comp,
		patch:           patch,
		compositeCRD:    compositeCRD,
		compositeResGVK: compositeResGVK,
		resourceCRD:     resourceCRD,
		resourceGVK:     resourceGVK,
	}), field.NewPath("spec").Child("resources").Index(resourceNumber).Child("patches").Index(patchNumber))
}

type patchValidationCtx struct {
	comp            *v1.Composition
	patch           v1.Patch
	compositeCRD    *apiextensions.CustomResourceDefinition
	compositeResGVK schema.GroupVersionKind
	resourceCRD     *apiextensions.CustomResourceDefinition
	resourceGVK     schema.GroupVersionKind
}

func (v *Validator) validatePatchWithSchemaInternal(ctx patchValidationCtx) *field.Error { //nolint:gocyclo // mainly due to the switch, not much to refactor
	var validationErr *field.Error
	var fromType, toType xpschema.KnownJSONType
	switch ctx.patch.GetType() {
	case v1.PatchTypeFromCompositeFieldPath:
		fromType, toType, validationErr = validateFromCompositeFieldPathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.compositeCRD, ctx.compositeResGVK.Version),
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
		)
	case v1.PatchTypeToCompositeFieldPath:
		fromType, toType, validationErr = validateFromCompositeFieldPathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
			getSchemaForVersion(ctx.compositeCRD, ctx.compositeResGVK.Version),
		)
	case v1.PatchTypeCombineFromComposite:
		fromType, toType, validationErr = validateCombineFromCompositePathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.compositeCRD, ctx.compositeResGVK.Version),
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
		)
	case v1.PatchTypeCombineToComposite:
		fromType, toType, validationErr = validateCombineFromCompositePathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
			getSchemaForVersion(ctx.compositeCRD, ctx.compositeResGVK.Version),
		)
	case v1.PatchTypePatchSet:
		// patches in a patch set are validated separately, so we'll just recurse one level deeper
		for i, ps := range ctx.comp.Spec.PatchSets {
			if *ctx.patch.PatchSetName == ps.Name {
				for j, patch := range ps.Patches {
					if err := v.validatePatchWithSchemaInternal(patchValidationCtx{
						comp:            ctx.comp,
						patch:           patch,
						compositeCRD:    ctx.compositeCRD,
						compositeResGVK: ctx.compositeResGVK,
						resourceCRD:     ctx.resourceCRD,
						resourceGVK:     ctx.resourceGVK,
					},
					); err != nil {
						return verrors.WrapFieldError(err, field.NewPath("patchSets").Index(i).Child("patches").Index(j))
					}
				}
			}
		}
	case v1.PatchTypeFromEnvironmentFieldPath:
		fromType, toType, validationErr = validateFromCompositeFieldPathPatch(
			ctx.patch,
			nil,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
		)
	case v1.PatchTypeToEnvironmentFieldPath:
		fromType, toType, validationErr = validateFromCompositeFieldPathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
			nil,
		)
	case v1.PatchTypeCombineFromEnvironment:
		fromType, toType, validationErr = validateCombineFromCompositePathPatch(
			ctx.patch,
			nil,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
		)
	case v1.PatchTypeCombineToEnvironment:
		fromType, toType, validationErr = validateCombineFromCompositePathPatch(
			ctx.patch,
			getSchemaForVersion(ctx.resourceCRD, ctx.resourceGVK.Version),
			nil,
		)
	}
	if validationErr != nil {
		return validationErr
	}
	return validateIOTypesWithTransforms(ctx.patch.Transforms, fromType, toType)
}

// validateCombineFromCompositePathPatch validates Combine Patch types, by going through and validating the fromField
// path variables, checking if the right combine strategy is set and validating transforms.
func validateCombineFromCompositePathPatch(patch v1.Patch, from, to *apiextensions.JSONSchemaProps) (fromType, toType xpschema.KnownJSONType, err *field.Error) {
	toFieldPath := patch.GetToFieldPath()
	toType, toFieldPathErr := validateFieldPath(to, toFieldPath)
	if toFieldPathErr != nil {
		return "", "", field.Invalid(field.NewPath("toFieldPath"), toFieldPath, toFieldPathErr.Error())
	}
	errs := field.ErrorList{}
	for _, variable := range patch.Combine.Variables {
		fromFieldPath := variable.FromFieldPath
		_, err := validateFieldPath(from, fromFieldPath)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("fromFieldPath"), fromFieldPath, err.Error()))
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

	// TODO(lsviben): check if we could validate the patch combine format, worth looking at https://cs.opensource.google/go/x/tools/+/refs/tags/v0.7.0:go/analysis/passes/printf/printf.go;l=1025

	return fromType, toType, nil
}

// validateFromCompositeFieldPathPatch validates a patch of type FromCompositeFieldPath.
func validateFromCompositeFieldPathPatch(patch v1.Patch, from, to *apiextensions.JSONSchemaProps) (fromType, toType xpschema.KnownJSONType, res *field.Error) {
	fromFieldPath := patch.GetFromFieldPath()
	toFieldPath := patch.GetToFieldPath()
	fromType, err := validateFieldPath(from, fromFieldPath)
	if err != nil {
		return "", "", field.Invalid(field.NewPath("fromFieldPath"), fromFieldPath, err.Error())
	}

	toType, err = validateFieldPath(to, toFieldPath)
	if err != nil {
		return "", "", field.Invalid(field.NewPath("toFieldPath"), toFieldPath, err.Error())
	}

	return fromType, toType, nil
}

func validateIOTypesWithTransforms(transforms []v1.Transform, fromType, toType xpschema.KnownJSONType) *field.Error {
	// if there are no transforms and the types are either the same or unknown, we don't need to validate transforms
	if len(transforms) == 0 && (fromType == "" || toType == "" || fromType == toType) {
		return nil
	}

	transformsOutputType, fieldErr := validateTransformsChainIOTypes(transforms, fromType)
	if fieldErr != nil {
		return fieldErr
	}

	if transformsOutputType == "" || toType == "" || xpschema.FromTransformIOType(transformsOutputType).IsEquivalent(toType) {
		return nil
	}

	if len(transforms) == 0 {
		return field.Required(field.NewPath("transforms"), fmt.Sprintf("the fromFieldPath does not have a type compatible with the toFieldPath according to their schemas and no transforms were provided: %s != %s", fromType, toType))
	}
	return field.Invalid(field.NewPath("transforms"), transforms, fmt.Sprintf("the provided transforms do not output a type compatible with the toFieldPath according to the schema: %s != %s", fromType, toType))
}

func validateTransformsChainIOTypes(transforms []v1.Transform, fromType xpschema.KnownJSONType) (v1.TransformIOType, *field.Error) {
	inputType, err := xpschema.FromKnownJSONType(fromType)
	if err != nil && fromType != "" {
		return "", field.InternalError(field.NewPath("transforms"), err)
	}
	for i, transform := range transforms {
		transform := transform
		err := IsValidInputForTransform(&transform, inputType)
		if err != nil && inputType != "" {
			return "", field.Invalid(field.NewPath("transforms").Index(i), transform, err.Error())
		}
		out, err := transform.GetOutputType()
		if err != nil {
			return "", field.InternalError(field.NewPath("transforms").Index(i), err)
		}
		if out == nil {
			// no need to validate the rest of the transforms as a nil output without error means we don't
			// have a way to know the output type for some transforms
			return "", nil
		}
		inputType = *out
	}
	return inputType, nil
}

// validateFieldPath validates the given fieldPath is valid for the given schema.
// It returns the type of the fieldPath and any error.
// If the returned type is "", but without error, it means the fieldPath is accepted by the schema, but not defined in it.
func validateFieldPath(schema *apiextensions.JSONSchemaProps, fieldPath string) (fieldType xpschema.KnownJSONType, err error) {
	// Code inspired by crossplane-contrib/crossplane-lint implementation:
	// https://github.com/crossplane-contrib/crossplane-lint/commit/d58af636f06467151cce7c89ffd319828c1cd7a2#diff-3b13ed191dd7244f19f4c0870298fc5112153e136250e95095323e6c3c440bdfR230
	if fieldPath == "" {
		return "", nil
	}
	segments, err := fieldpath.Parse(fieldPath)
	if err != nil {
		return "", err
	}
	if len(segments) > 0 && segments[0].Type == fieldpath.SegmentField && segments[0].Field == "metadata" {
		// if the fieldPath starts with metadata, we need to merge the metadata schema with the schema
		// to make sure we validate the fieldPath correctly.
		schema = defaultMetadataSchema(schema)
	}
	return validateFieldPathSegments(segments, schema, fieldPath)
}

func validateFieldPathSegments(segments fieldpath.Segments, schema *apiextensions.JSONSchemaProps, fieldPath string) (xpschema.KnownJSONType, error) {
	current := schema
	for _, segment := range segments {
		currentSegment, err := validateFieldPathSegment(current, segment)
		if err != nil {
			return "", err
		}
		if currentSegment == nil {
			return "", nil
		}
		current = currentSegment
	}

	if !xpschema.IsValid(current.Type) {
		return "", fmt.Errorf("field path %q has an unsupported type %q", fieldPath, current.Type)
	}
	return xpschema.KnownJSONType(current.Type), nil
}

// validateFieldPathSegment validates that the given field path segment is valid for the given schema.
// It returns the schema for the segment, and an error if the segment is invalid.
func validateFieldPathSegment(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (current *apiextensions.JSONSchemaProps, err error) {
	switch segment.Type {
	case fieldpath.SegmentField:
		return validateFieldPathSegmentField(parent, segment)
	case fieldpath.SegmentIndex:
		return validateFieldPathSegmentIndex(parent, segment)
	}
	return nil, nil
}

func validateFieldPathSegmentField(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (*apiextensions.JSONSchemaProps, error) { //nolint:gocyclo // inherently complex
	if parent == nil {
		return nil, nil
	}
	if segment.Type != fieldpath.SegmentField {
		return nil, errors.Errorf("segment is not a field")
	}
	if propType := parent.Type; propType != "" && propType != string(xpschema.KnownJSONTypeObject) {
		return nil, errors.Errorf(errFmtFieldAccessWrongType, segment.Field, propType)
	}
	// TODO(phisco): any remaining fields? e.g. XValidations' CEL Rules?
	prop, exists := parent.Properties[segment.Field]
	if !exists {
		if ptr.Deref(parent.XPreserveUnknownFields, false) {
			return nil, nil
		}

		// Allows and Schema are mutually exclusive, so we should accept additional properties both if Allows is true or
		// Schema is not nil.
		// See https://github.com/kubernetes/kubernetes/blob/ff4eff24ac4fad5431aa89681717d6c4fe5733a4/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go#L828
		if parent.AdditionalProperties != nil && (parent.AdditionalProperties.Allows || parent.AdditionalProperties.Schema != nil) {
			if parent.AdditionalProperties.Schema != nil && parent.AdditionalProperties.Schema.Type != "" {
				return parent.AdditionalProperties.Schema, nil
			}
			// re-evaluate the segment against the additional properties schema
			return validateFieldPathSegmentField(parent.AdditionalProperties.Schema, segment)
		}
		return nil, errors.Errorf(errFmtFieldInvalid, segment.Field)

	}
	return &prop, nil
}

func validateFieldPathSegmentIndex(parent *apiextensions.JSONSchemaProps, segment fieldpath.Segment) (*apiextensions.JSONSchemaProps, error) {
	if parent == nil {
		return nil, nil
	}
	if segment.Type != fieldpath.SegmentIndex {
		return nil, errors.Errorf("segment is not an index")
	}
	if parent.Type != string(xpschema.KnownJSONTypeArray) {
		return nil, errors.Errorf(errFmtIndexAccessWrongType, parent.Type)
	}
	if parent.Items == nil {
		return nil, errors.New("no items found in array")
	}
	// if there is a limit on max items and the index is above that, return an error
	if parent.MaxItems != nil && *parent.MaxItems < int64(segment.Index+1) {
		return nil, errors.Errorf(errFmtArrayIndexAboveMax, segment.Index, *parent.MaxItems-1)
	}
	if s := parent.Items.Schema; s != nil {
		return s, nil
	}
	schemas := parent.Items.JSONSchemas
	if len(schemas) < int(segment.Index) {
		return nil, errors.Errorf("no schema for item requested at index %d", segment.Index)
	}

	// means there is no schema at all for this array
	return nil, nil
}

// IsValidInputForTransform validates the supplied Transform type, taking into consideration also the input type.
//
//nolint:gocyclo // This is a long but simple/same-y switch.
func IsValidInputForTransform(t *v1.Transform, fromType v1.TransformIOType) error {
	switch t.Type {
	case v1.TransformTypeMath:
		if fromType != v1.TransformIOTypeInt && fromType != v1.TransformIOTypeInt64 && fromType != v1.TransformIOTypeFloat64 {
			return errors.Errorf("math transform can only be used with numeric types, got %s", fromType)
		}
	case v1.TransformTypeMap:
		if fromType != v1.TransformIOTypeString {
			return errors.Errorf("map transform can only be used with string types, got %s", fromType)
		}
	case v1.TransformTypeMatch:
		if fromType != v1.TransformIOTypeString {
			return errors.Errorf("match transform can only be used with string input types, got %s", fromType)
		}
	case v1.TransformTypeString:
		switch t.String.Type {
		case v1.StringTransformTypeRegexp, v1.StringTransformTypeTrimSuffix, v1.StringTransformTypeTrimPrefix:
			if fromType != v1.TransformIOTypeString {
				return errors.Errorf("string transform can only be used with string input types, got %s", fromType)
			}
		case v1.StringTransformTypeFormat:
			// any input type is valid
		case v1.StringTransformTypeConvert:
			if t.String.Convert == nil {
				return errors.Errorf("string transform convert type is required for convert transform")
			}
			switch *t.String.Convert {
			case v1.StringConversionTypeToLower, v1.StringConversionTypeToUpper, v1.StringConversionTypeFromBase64, v1.StringConversionTypeToBase64:
				if fromType != v1.TransformIOTypeString {
					return errors.Errorf("string transform can only be used with string input types, got %s", fromType)
				}
			case v1.StringConversionTypeToJSON, v1.StringConversionTypeToAdler32, v1.StringConversionTypeToSHA1, v1.StringConversionTypeToSHA256, v1.StringConversionTypeToSHA512:
				// any input type is valid
			default:
				return errors.Errorf("unknown string conversion type %s", *t.String.Convert)
			}
		case v1.StringTransformTypeJoin:
			if fromType != v1.TransformIOTypeArray {
				return errors.Errorf("string transform join type can only be used with arrays as input, got %s", fromType)
			}
		default:
			return errors.Errorf("unknown string transform type %s", t.String.Type)
		}
	case v1.TransformTypeConvert:
		if _, err := composite.GetConversionFunc(t.Convert, fromType); err != nil {
			return err
		}
	default:
		return errors.Errorf("unknown transform type %s", t.Type)
	}
	return nil
}

// GetBaseObject returns the base object of the composed template.
// Uses the cached object if it is available, or parses the raw Base
// otherwise. The returned object is a deep copy.
func GetBaseObject(ct *v1.ComposedTemplate) (client.Object, error) {
	if ct.Base.Object == nil {
		cd := composed.New()
		err := json.Unmarshal(ct.Base.Raw, cd)
		if err != nil {
			return nil, errors.Wrap(err, errUnableToParse)
		}
		ct.Base.Object = cd
	}
	if ct, ok := ct.Base.Object.(client.Object); ok {
		return ct.DeepCopyObject().(client.Object), nil
	}
	return nil, errors.New("base object is not a client.Object")
}

// GetBaseObjectGVK returns the GroupVersionKind of the base object of the composed template.
func GetBaseObjectGVK(ct *v1.ComposedTemplate) (schema.GroupVersionKind, error) {
	obj, err := GetBaseObject(ct)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return obj.GetObjectKind().GroupVersionKind(), nil
}

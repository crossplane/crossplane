/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"context"
	"fmt"
	"io"

	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
)

const (
	errWriteOutput = "cannot write output"
)

func newValidatorsAndStructurals(crds []*extv1.CustomResourceDefinition) (map[runtimeschema.GroupVersionKind][]*validation.SchemaValidator, map[runtimeschema.GroupVersionKind]*schema.Structural, error) {
	validators := map[runtimeschema.GroupVersionKind][]*validation.SchemaValidator{}
	structurals := map[runtimeschema.GroupVersionKind]*schema.Structural{}

	for i := range crds {
		internal := &ext.CustomResourceDefinition{}
		if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crds[i], internal, nil); err != nil {
			return nil, nil, err
		}

		// Top-level and per-version schemas are mutually exclusive.
		for _, ver := range internal.Spec.Versions {
			var sv validation.SchemaValidator
			var err error

			gvk := runtimeschema.GroupVersionKind{
				Group:   internal.Spec.Group,
				Version: ver.Name,
				Kind:    internal.Spec.Names.Kind,
			}

			var s *ext.JSONSchemaProps
			switch {
			case internal.Spec.Validation != nil:
				s = internal.Spec.Validation.OpenAPIV3Schema
			case ver.Schema != nil && ver.Schema.OpenAPIV3Schema != nil:
				s = ver.Schema.OpenAPIV3Schema
			default:
				// TODO log a warning here, it should never happen
				continue
			}

			sv, _, err = validation.NewSchemaValidator(s)
			if err != nil {
				return nil, nil, err
			}

			validators[gvk] = append(validators[gvk], &sv)

			structural, err := schema.NewStructural(s)
			if err != nil {
				return nil, nil, err
			}

			structurals[gvk] = structural
		}
	}

	return validators, structurals, nil
}

// SchemaValidation validates the resources against the given CRDs.
func SchemaValidation(resources []*unstructured.Unstructured, crds []*extv1.CustomResourceDefinition, skipSuccessLogs bool, w io.Writer) error { //nolint:gocognit // printing the output increases the cyclomatic complexity a little bit
	schemaValidators, structurals, err := newValidatorsAndStructurals(crds)
	if err != nil {
		return errors.Wrap(err, "cannot create schema validators")
	}

	failure, missingSchemas := 0, 0

	for i, r := range resources {
		gvk := r.GetObjectKind().GroupVersionKind()
		sv, ok := schemaValidators[gvk]
		s := structurals[gvk] // if we have a schema validator, we should also have a structural
		if !ok {
			missingSchemas++
			if _, err := fmt.Fprintf(w, "[!] could not find CRD/XRD for: %s\n", r.GroupVersionKind().String()); err != nil {
				return errors.Wrap(err, errWriteOutput)
			}

			continue
		}

		rf := 0
		re := field.ErrorList{}
		for _, v := range sv {
			re = append(re, validation.ValidateCustomResource(nil, r, *v)...)
			re = append(re, validateUnknownFields(r.UnstructuredContent(), s)...)
			for _, e := range re {
				rf++
				if _, err := fmt.Fprintf(w, "[x] schema validation error %s, %s : %s\n", r.GroupVersionKind().String(), getResourceName(r), e.Error()); err != nil {
					return errors.Wrap(err, errWriteOutput)
				}
			}

			celValidator := cel.NewValidator(s, true, celconfig.PerCallLimit)
			re, _ = celValidator.Validate(context.TODO(), nil, s, resources[i].Object, nil, celconfig.PerCallLimit)
			for _, e := range re {
				rf++
				if _, err := fmt.Fprintf(w, "[x] CEL validation error %s, %s : %s\n", r.GroupVersionKind().String(), getResourceName(r), e.Error()); err != nil {
					return errors.Wrap(err, errWriteOutput)
				}
			}

			if rf == 0 && !skipSuccessLogs {
				if _, err := fmt.Fprintf(w, "[✓] %s, %s validated successfully\n", r.GroupVersionKind().String(), getResourceName(r)); err != nil {
					return errors.Wrap(err, errWriteOutput)
				}
			} else {
				failure++
			}
		}
	}

	if _, err := fmt.Fprintf(w, "Total %d resources: %d missing schemas, %d success cases, %d failure cases\n", len(resources), missingSchemas, len(resources)-failure-missingSchemas, failure); err != nil {
		return errors.Wrap(err, errWriteOutput)
	}

	if failure > 0 {
		return errors.New("could not validate all resources")
	}

	return nil
}

func getResourceName(r *unstructured.Unstructured) string {
	if r.GetName() != "" {
		return r.GetName()
	}

	// fallback to composition resource name
	return r.GetAnnotations()[composite.AnnotationKeyCompositionResourceName]
}

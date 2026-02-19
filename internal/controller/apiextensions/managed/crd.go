/*
Copyright 2025 The Crossplane Authors.

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

package managed

import (
	"encoding/json"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
)

// CRDAsUnstructured builds an Unstructured CRD from an MRD.
// This is used for server-side apply to ensure we only serialize the fields we
// have opinions about, avoiding issues with zero values and defaults.
func CRDAsUnstructured(mrd *v1alpha1.ManagedResourceDefinition) (*unstructured.Unstructured, error) {
	want := mrd.Spec.CustomResourceDefinitionSpec

	// Start with a Paved object for easier field manipulation
	p := fieldpath.Pave(map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name": mrd.GetName(),
		},
	})

	// Set required spec fields. We ignore errors because they can only occur
	// if the path is malformed, which is impossible with our hardcoded paths.
	_ = p.SetString("spec.group", want.Group)
	_ = p.SetString("spec.scope", string(want.Scope))

	// Set names
	_ = p.SetString("spec.names.kind", want.Names.Kind)
	_ = p.SetString("spec.names.plural", want.Names.Plural)
	if want.Names.Singular != "" {
		_ = p.SetString("spec.names.singular", want.Names.Singular)
	}
	if want.Names.ListKind != "" {
		_ = p.SetString("spec.names.listKind", want.Names.ListKind)
	}
	if len(want.Names.ShortNames) > 0 {
		_ = p.SetValue("spec.names.shortNames", want.Names.ShortNames)
	}
	if len(want.Names.Categories) > 0 {
		_ = p.SetValue("spec.names.categories", want.Names.Categories)
	}

	// Set optional spec fields
	if want.Conversion != nil {
		_ = p.SetValue("spec.conversion", want.Conversion)
	}
	if want.PreserveUnknownFields {
		_ = p.SetBool("spec.preserveUnknownFields", want.PreserveUnknownFields)
	}

	// Build versions
	versions := make([]map[string]any, len(want.Versions))
	for i, version := range want.Versions {
		v, err := buildVersion(version)
		if err != nil {
			return nil, err
		}
		versions[i] = v
	}

	// Sort versions by name for deterministic output
	slices.SortFunc(versions, func(a, b map[string]any) int {
		return strings.Compare(a["name"].(string), b["name"].(string)) //nolint:forcetypeassert // Guaranteed to be string.
	})

	_ = p.SetValue("spec.versions", versions)

	// Build the Unstructured from the Paved content
	u := &unstructured.Unstructured{Object: p.UnstructuredContent()}

	// Add owner references
	meta.AddOwnerReference(u, meta.AsOwner(meta.TypedReferenceTo(mrd, v1alpha1.ManagedResourceDefinitionGroupVersionKind)))
	if owner := metav1.GetControllerOf(mrd); owner != nil {
		meta.AddOwnerReference(u, *owner)
	}

	return u, nil
}

// buildVersion builds a version map for a CRD version specification.
func buildVersion(version v1alpha1.CustomResourceDefinitionVersion) (map[string]any, error) {
	p := fieldpath.Pave(map[string]any{})

	// Set required fields
	_ = p.SetString("name", version.Name)
	_ = p.SetBool("served", version.Served)
	_ = p.SetBool("storage", version.Storage)

	// Set optional fields only if non-zero to avoid zero-value serialization
	if version.Deprecated {
		_ = p.SetBool("deprecated", version.Deprecated)
	}
	if version.DeprecationWarning != nil {
		_ = p.SetString("deprecationWarning", *version.DeprecationWarning)
	}

	if version.Schema != nil {
		var schema map[string]any
		if err := json.Unmarshal(version.Schema.OpenAPIV3Schema.Raw, &schema); err != nil {
			return nil, errors.Wrapf(err, "cannot parse OpenAPI v3 schema for version %q", version.Name)
		}
		_ = p.SetValue("schema.openAPIV3Schema", schema)
	}

	if version.Subresources != nil {
		if version.Subresources.Status != nil {
			_ = p.SetValue("subresources.status", map[string]any{})
		}
		if version.Subresources.Scale != nil {
			_ = p.SetValue("subresources.scale", version.Subresources.Scale)
		}
	}

	if len(version.AdditionalPrinterColumns) > 0 {
		_ = p.SetValue("additionalPrinterColumns", version.AdditionalPrinterColumns)
	}

	if len(version.SelectableFields) > 0 {
		_ = p.SetValue("selectableFields", version.SelectableFields)
	}

	return p.UnstructuredContent(), nil
}

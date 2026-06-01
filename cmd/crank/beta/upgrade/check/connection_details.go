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

package check

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// CompositeConnectionDetails finds Compositions, XRs, and Claims that rely on
// built-in composite resource connection details, which are removed in
// Crossplane v2 for v2 XRs. This support does still exist in v2 for legacy XRs,
// so this category is considered informational.
type CompositeConnectionDetails struct {
	Client client.Client
	// Namespace, if non-empty, restricts Claim instance lookups to a single
	// namespace. Empty means list across all namespaces.
	Namespace string
}

// Meta returns the check's static metadata.
func (c *CompositeConnectionDetails) Meta() Meta {
	return Meta{
		Category:    "composite-connection-details",
		Title:       "Composite resource connection details",
		Severity:    SeverityInfo,
		Description: "Crossplane v2 keeps legacy XR and Claim connection-detail aggregation working, so no action is required for the upgrade itself. This check is informational: it flags resources that would need explicit composed Secrets if you later migrate them to v2-style namespaced XRs. Managed resources are unaffected and continue to publish via spec.writeConnectionSecretToRef.",
		Remediation: "No action required for the upgrade. If you later migrate these XRDs to v2-style namespaced XRs, compose a Kubernetes Secret explicitly to publish connection details. No automated converter exists.",
		DocsURLs: []string{
			"https://docs.crossplane.io/latest/guides/upgrade-to-crossplane-v2/#composite-resource-connection-details",
			"https://docs.crossplane.io/latest/guides/connection-details-composition",
		},
	}
}

// Run emits a Finding for each XRD, Composition, XR, and Claim that relies on
// built-in composite resource connection-detail aggregation.
func (c *CompositeConnectionDetails) Run(ctx context.Context) ([]Finding, error) {
	// list all compositions
	comps := &apiextensionsv1.CompositionList{}
	if err := c.Client.List(ctx, comps); err != nil {
		return nil, errors.Wrap(err, "cannot list Compositions")
	}

	group := apiextensionsv1.Group

	findings := make([]Finding, 0, len(comps.Items))
	for i := range comps.Items {
		comp := &comps.Items[i]
		ref := ResourceRef{
			Group: group,
			Kind:  apiextensionsv1.CompositionKind,
			Name:  comp.Name,
		}

		// check the composition for writeConnectionSecretsToNamespace usage
		if comp.Spec.WriteConnectionSecretsToNamespace != nil && *comp.Spec.WriteConnectionSecretsToNamespace != "" {
			findings = append(findings, Finding{
				Resource:  ref,
				FieldPath: ".spec.writeConnectionSecretsToNamespace",
			})
		}

		// check the composition for native p-and-t connectionDetails usage
		if nativePublishesConnectionDetails(comp) {
			findings = append(findings, Finding{
				Resource:  ref,
				FieldPath: ".spec.resources[].connectionDetails",
			})
		}

		// check the composition for function p-and-t connectionDetails usage
		usesConnectionDetails, err := pipelinePublishesConnectionDetails(comp)
		if err != nil {
			return findings, errors.Wrapf(err, "cannot inspect Composition %s pipeline", comp.Name)
		}
		if usesConnectionDetails {
			findings = append(findings, Finding{
				Resource:  ref,
				FieldPath: ".spec.pipeline[].input.resources[].connectionDetails",
			})
		}
	}

	xrds := &apiextensionsv1.CompositeResourceDefinitionList{}
	if err := c.Client.List(ctx, xrds); err != nil {
		return findings, errors.Wrap(err, "cannot list CompositeResourceDefinitions")
	}
	for i := range xrds.Items {
		xrd := &xrds.Items[i]
		// check the XRD for connectionSecretKeys usage
		if len(xrd.Spec.ConnectionSecretKeys) == 0 {
			continue
		}
		findings = append(findings, Finding{
			Resource: ResourceRef{
				Group: group,
				Kind:  apiextensionsv1.CompositeResourceDefinitionKind,
				Name:  xrd.Name,
			},
			FieldPath: ".spec.connectionSecretKeys",
		})
	}

	// find all XR and Claim types
	types, err := DiscoverXRAndClaimTypes(ctx, c.Client, xrds)
	if err != nil {
		return findings, errors.Wrap(err, "cannot discover XR and Claim types")
	}

	for _, t := range types {
		// list all instances of each XR/Claim type
		instances, err := ListInstances(ctx, c.Client, t, c.Namespace)
		if err != nil {
			return findings, errors.Wrapf(err, "cannot list instances of %s", t.GVK.String())
		}
		for i := range instances {
			// check the claim/XR for writeConnectionSecretToRef usage
			inst := instances[i]
			ref, found, err := unstructured.NestedMap(inst.Object, "spec", "writeConnectionSecretToRef")
			if err != nil {
				return findings, errors.Wrapf(err, "cannot read spec.writeConnectionSecretToRef on %s/%s", inst.GetKind(), inst.GetName())
			}
			if !found || ref == nil {
				continue
			}
			findings = append(findings, Finding{
				Resource:  ResourceRefFromUnstructured(inst),
				FieldPath: ".spec.writeConnectionSecretToRef",
			})
		}
	}

	return findings, nil
}

// nativePublishesConnectionDetails reports whether the composition declares
// connectionDetails on any native patch and transform (mode: Resources)
// composed resource. If the composition isn't using native p-and-t then this
// loop will have nothing to iterate over and that's fine.
func nativePublishesConnectionDetails(comp *apiextensionsv1.Composition) bool {
	for i := range comp.Spec.Resources {
		if len(comp.Spec.Resources[i].ConnectionDetails) > 0 {
			return true
		}
	}
	return false
}

// pipelinePublishesConnectionDetails reports whether any
// function-patch-and-transform step in the composition declares
// connectionDetails on a composed resource. Only inputs in the
// function-patch-and-transform API group are inspected; other functions carry
// their own input schemas we can't interpret. If the composition isn't using
// pipeline mode then this function will have nothing to iterate over and that's
// fine.
func pipelinePublishesConnectionDetails(comp *apiextensionsv1.Composition) (bool, error) {
	for i := range comp.Spec.Pipeline {
		step := &comp.Spec.Pipeline[i]
		if step.Input == nil || len(step.Input.Raw) == 0 {
			continue
		}
		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(step.Input.Raw); err != nil {
			return false, errors.Wrapf(err, "cannot parse input of pipeline step %q", step.Step)
		}
		if u.GroupVersionKind().Group != "pt.fn.crossplane.io" {
			// not a function-patch-and-transform pipeline step, skip it
			continue
		}
		resources, _, _ := unstructured.NestedSlice(u.Object, "resources")
		for _, r := range resources {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if cd, found, _ := unstructured.NestedSlice(rm, "connectionDetails"); found && len(cd) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

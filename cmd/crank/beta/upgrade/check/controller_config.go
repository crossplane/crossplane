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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

// ControllerConfigCheck finds usage of the removed ControllerConfig type:
// ControllerConfig CRs themselves and Providers/Functions that still reference
// one via spec.controllerConfigRef.
type ControllerConfigCheck struct {
	Client client.Client
}

// Meta returns the check's static metadata.
func (c *ControllerConfigCheck) Meta() Meta {
	return Meta{
		Category:    "controller-config",
		Title:       "ControllerConfig usage",
		Severity:    SeverityIssue,
		Description: "Crossplane v2 removes the ControllerConfig type. Use DeploymentRuntimeConfig instead.",
		Remediation: `Migrate to DeploymentRuntimeConfig (pkg.crossplane.io/v1beta1). Run "crossplane beta convert deployment-runtime" to convert existing ControllerConfigs.`,
		DocsURLs: []string{
			"https://docs.crossplane.io/latest/guides/upgrade-to-crossplane-v2/#controllerconfig-type",
			"https://docs.crossplane.io/v1.20/cli/command-reference/#beta-convert",
		},
	}
}

// Run emits a Finding for each ControllerConfig CR and each Provider or
// Function whose spec.controllerConfigRef is set.
func (c *ControllerConfigCheck) Run(ctx context.Context) ([]Finding, error) {
	// list all ControllerConfigs and report all as findings
	ccs := &pkgv1alpha1.ControllerConfigList{}
	if err := c.Client.List(ctx, ccs); err != nil {
		return nil, errors.Wrap(err, "cannot list ControllerConfigs")
	}

	// ControllerConfig and Provider/Function share the same Group
	// (pkg.crossplane.io); only the version differs and version is no longer
	// part of the ResourceRef identity.
	group := pkgv1.Group

	findings := make([]Finding, 0, len(ccs.Items))
	for i := range ccs.Items {
		cc := &ccs.Items[i]
		findings = append(findings, Finding{
			Resource: ResourceRef{
				Group: group,
				Kind:  pkgv1alpha1.ControllerConfigKind,
				Name:  cc.Name,
			},
		})
	}

	// list all providers and find all that are referencing ControllerConfigs
	providers := &pkgv1.ProviderList{}
	if err := c.Client.List(ctx, providers); err != nil {
		return findings, errors.Wrap(err, "cannot list Providers")
	}
	for i := range providers.Items {
		p := &providers.Items[i]
		if p.Spec.ControllerConfigReference == nil { //nolint:staticcheck // intentional read of deprecated field
			continue
		}
		findings = append(findings, Finding{
			Resource: ResourceRef{
				Group: group,
				Kind:  pkgv1.ProviderKind,
				Name:  p.Name,
			},
			FieldPath: ".spec.controllerConfigRef",
		})
	}

	// list all functions and find all that are referencing ControllerConfigs
	functions := &pkgv1.FunctionList{}
	if err := c.Client.List(ctx, functions); err != nil {
		return findings, errors.Wrap(err, "cannot list Functions")
	}
	for i := range functions.Items {
		f := &functions.Items[i]
		if f.Spec.ControllerConfigReference == nil { //nolint:staticcheck // intentional read of deprecated field
			continue
		}
		findings = append(findings, Finding{
			Resource: ResourceRef{
				Group: group,
				Kind:  pkgv1.FunctionKind,
				Name:  f.Name,
			},
			FieldPath: ".spec.controllerConfigRef",
		})
	}

	return findings, nil
}

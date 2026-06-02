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

	"github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// UnqualifiedPackageSources finds Providers, Configurations, and Functions
// whose spec.package is not fully qualified with a registry hostname.
// Crossplane v2 removes the default registry and the --registry flag, so every
// package reference must include its registry explicitly.
type UnqualifiedPackageSources struct {
	Client client.Client
}

// Meta returns the check's static metadata.
func (c *UnqualifiedPackageSources) Meta() Meta {
	return Meta{
		Category:    "unqualified-package-source",
		Title:       "Unqualified package sources",
		Severity:    SeverityIssue,
		Description: "Crossplane v2 removes the --registry flag and its implicit default registry. Every Provider, Configuration, and Function spec.package (including dependencies) must be a fully qualified reference including its registry hostname.",
		Remediation: "Prefix the package with its fully qualified registry hostname (e.g. xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.4.0) and ensure all package references are valid.",
		DocsURLs:    []string{"https://docs.crossplane.io/latest/guides/upgrade-to-crossplane-v2/#default-registry-flag"},
	}
}

// Run lists all package types and emits a Finding for each one whose
// spec.package is not a fully qualified registry reference.
func (c *UnqualifiedPackageSources) Run(ctx context.Context) ([]Finding, error) {
	var findings []Finding
	group := pkgv1.Group

	// check package sources for all providers
	providers := &pkgv1.ProviderList{}
	if err := c.Client.List(ctx, providers); err != nil {
		return findings, errors.Wrap(err, "cannot list Providers")
	}
	for i := range providers.Items {
		p := &providers.Items[i]
		if f := checkPackage(p.Spec.Package, ResourceRef{
			Group: group,
			Kind:  pkgv1.ProviderKind,
			Name:  p.Name,
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	// check package sources for all configurations
	configurations := &pkgv1.ConfigurationList{}
	if err := c.Client.List(ctx, configurations); err != nil {
		return findings, errors.Wrap(err, "cannot list Configurations")
	}
	for i := range configurations.Items {
		cfg := &configurations.Items[i]
		if f := checkPackage(cfg.Spec.Package, ResourceRef{
			Group: group,
			Kind:  pkgv1.ConfigurationKind,
			Name:  cfg.Name,
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	// check package sources for all functions
	functions := &pkgv1.FunctionList{}
	if err := c.Client.List(ctx, functions); err != nil {
		return findings, errors.Wrap(err, "cannot list Functions")
	}
	for i := range functions.Items {
		fn := &functions.Items[i]
		if f := checkPackage(fn.Spec.Package, ResourceRef{
			Group: group,
			Kind:  pkgv1.FunctionKind,
			Name:  fn.Name,
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings, nil
}

// checkPackage returns a Finding when pkg is missing a registry hostname or
// can't be parsed at all.
func checkPackage(pkg string, ref ResourceRef) *Finding {
	// Parse with an empty default registry so ParseReference doesn't add a
	// registry that isn't explicitly declared in the reference we're checking.
	parsed, err := name.ParseReference(pkg, name.WithDefaultRegistry(""))
	if err == nil && parsed.Context().RegistryStr() != "" {
		return nil
	}

	fieldPath := ".spec.package"
	if err != nil {
		// Distinguish "parses but no registry" from "doesn't parse at all" in
		// the finding so the user can tell what they're looking at.
		fieldPath = ".spec.package (unparseable)"
	}

	return &Finding{Resource: ref, FieldPath: fieldPath}
}

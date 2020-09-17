/*
Copyright 2020 The Crossplane Authors.

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

package revision

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reference"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	errNotProvider              = "not a provider package"
	errDeleteProviderDeployment = "cannot delete provider package deployment"
	errDeleteProviderSA         = "cannot delete provider package service account"
	errApplyProviderDeployment  = "cannot apply provider package deployment"
	errApplyProviderSA          = "cannot apply provider package service account"

	errNotConfiguration = "not a configuration package"
)

// A Hook performs operations before and after a revision establishes
// objects.
type Hook interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1alpha1.PackageRevision) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1alpha1.PackageRevision) error
}

// ProviderHook preforms operations for a provider package that requires a
// controller before and after the revision establishes objects.
type ProviderHook struct {
	client    resource.ClientApplicator
	namespace string
}

// NewProviderHook creates a new ProviderHook.
func NewProviderHook(client resource.ClientApplicator, namespace string) *ProviderHook {
	return &ProviderHook{
		client:    client,
		namespace: namespace,
	}
}

// Pre cleans up a packaged controller and service account if the
// revision is inactive.
func (h *ProviderHook) Pre(ctx context.Context, pkg runtime.Object, pr v1alpha1.PackageRevision) error {
	pkgProvider, ok := pkg.(*pkgmeta.Provider)
	if !ok {
		return errors.New(errNotProvider)
	}
	// Always set revision status fields.
	pr.SetDependencies(convertDependencies(pkgProvider.Spec.DependsOn))
	pr.SetCrossplaneVersion(reference.FromPtrValue(pkgProvider.Spec.Crossplane))

	// Do not clean up SA and controller if revision is not inactive.
	if pr.GetDesiredState() != v1alpha1.PackageRevisionInactive {
		return nil
	}
	s, d := buildProviderDeployment(pkgProvider, pr, h.namespace)
	if err := h.client.Delete(ctx, d); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}
	if err := h.client.Delete(ctx, s); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}
	return nil
}

// Post creates a packaged provider controller and service account if the
// revision is active.
func (h *ProviderHook) Post(ctx context.Context, pkg runtime.Object, pr v1alpha1.PackageRevision) error {
	pkgProvider, ok := pkg.(*pkgmeta.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1alpha1.PackageRevisionActive {
		return nil
	}
	s, d := buildProviderDeployment(pkgProvider, pr, h.namespace)
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}
	pr.SetControllerReference(runtimev1alpha1.Reference{Name: d.GetName()})
	return nil
}

// ConfigurationHook preforms operations for a configuration package before and
// after the revision establishes objects.
type ConfigurationHook struct{}

// NewConfigurationHook creates a new ConfigurationHook.
func NewConfigurationHook() *ConfigurationHook {
	return &ConfigurationHook{}
}

// Pre sets status fields based on the configuration package.
func (h *ConfigurationHook) Pre(ctx context.Context, pkg runtime.Object, pr v1alpha1.PackageRevision) error {
	pkgConfig, ok := pkg.(*pkgmeta.Configuration)
	if !ok {
		return errors.New(errNotConfiguration)
	}
	// Always set revision status fields.
	pr.SetDependencies(convertDependencies(pkgConfig.Spec.DependsOn))
	pr.SetCrossplaneVersion(reference.FromPtrValue(pkgConfig.Spec.Crossplane))
	return nil
}

// Post is a no op for configuration packages.
func (h *ConfigurationHook) Post(context.Context, runtime.Object, v1alpha1.PackageRevision) error {
	return nil
}

// NopHook performs no operations.
type NopHook struct{}

// NewNopHook creates a hook that does nothing.
func NewNopHook() *NopHook {
	return &NopHook{}
}

// Pre does nothing and returns nil.
func (h *NopHook) Pre(context.Context, runtime.Object, v1alpha1.PackageRevision) error {
	return nil
}

// Post does nothing and returns nil.
func (h *NopHook) Post(context.Context, runtime.Object, v1alpha1.PackageRevision) error {
	return nil
}

// convertDependencies converts package meta dependencies to package revision
// dependencies.
func convertDependencies(deps []pkgmeta.Dependency) []v1alpha1.Dependency {
	dependsOn := make([]v1alpha1.Dependency, len(deps))
	for i, d := range deps {
		// Skip dependencies that are malformed.
		if (d.Configuration == nil && d.Provider == nil) || (d.Configuration != nil && d.Provider != nil) {
			continue
		}
		p := v1alpha1.Dependency{}
		if d.Configuration != nil {
			p.Package = *d.Configuration
		}
		if d.Provider != nil {
			p.Package = *d.Provider
		}
		p.Version = d.Version
		dependsOn[i] = p
	}
	return dependsOn
}

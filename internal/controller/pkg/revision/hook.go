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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	errNotProvider                   = "not a provider package"
	errNotProviderRevision           = "not a provider revision"
	errUpdateProviderRevision        = "cannot update provider revision"
	errControllerConfig              = "cannot get referenced controller config"
	errDeleteProviderDeployment      = "cannot delete provider package deployment"
	errDeleteProviderSA              = "cannot delete provider package service account"
	errApplyProviderDeployment       = "cannot apply provider package deployment"
	errApplyProviderSA               = "cannot apply provider package service account"
	errUnavailableProviderDeployment = "provider package deployment is unavailable"

	errNotConfiguration = "not a configuration package"
)

// A Hooks performs operations before and after a revision establishes objects.
type Hooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1beta1.PackageRevision) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1beta1.PackageRevision) error
}

// ProviderHooks performs operations for a provider package that requires a
// controller before and after the revision establishes objects.
type ProviderHooks struct {
	client    resource.ClientApplicator
	namespace string
}

// NewProviderHooks creates a new ProviderHooks.
func NewProviderHooks(client resource.ClientApplicator, namespace string) *ProviderHooks {
	return &ProviderHooks{
		client:    client,
		namespace: namespace,
	}
}

// Pre cleans up a packaged controller and service account if the revision is
// inactive.
func (h *ProviderHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1beta1.PackageRevision) error {
	pkgProvider, ok := pkg.(*pkgmeta.Provider)
	if !ok {
		return errors.New(errNotProvider)
	}

	provRev, ok := pr.(*v1beta1.ProviderRevision)
	if !ok {
		return errors.New(errNotProviderRevision)
	}

	if !cmp.Equal(provRev.Spec.PermissionRequests, pkgProvider.Spec.Controller.PermissionRequests) {
		provRev.Spec.PermissionRequests = pkgProvider.Spec.Controller.PermissionRequests
		if err := h.client.Update(ctx, provRev); err != nil {
			return errors.Wrap(err, errUpdateProviderRevision)
		}
	}

	// TODO(hasheddan): update any status fields relevant to package revisions.

	// Do not clean up SA and controller if revision is not inactive.
	if pr.GetDesiredState() != v1beta1.PackageRevisionInactive {
		return nil
	}
	cc, err := h.getControllerConfig(ctx, pr)
	if err != nil {
		return errors.Wrap(err, errControllerConfig)
	}
	s, d := buildProviderDeployment(pkgProvider, pr, cc, h.namespace)
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
func (h *ProviderHooks) Post(ctx context.Context, pkg runtime.Object, pr v1beta1.PackageRevision) error {
	pkgProvider, ok := pkg.(*pkgmeta.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1beta1.PackageRevisionActive {
		return nil
	}
	cc, err := h.getControllerConfig(ctx, pr)
	if err != nil {
		return errors.Wrap(err, errControllerConfig)
	}
	s, d := buildProviderDeployment(pkgProvider, pr, cc, h.namespace)
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}
	pr.SetControllerReference(xpv1.Reference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == v1.ConditionTrue {
				return nil
			}
			return errors.Errorf("%s: %s", errUnavailableProviderDeployment, c.Message)
		}
	}
	return nil
}

func (h *ProviderHooks) getControllerConfig(ctx context.Context, pr v1beta1.PackageRevision) (*v1alpha1.ControllerConfig, error) {
	var cc *v1alpha1.ControllerConfig
	if pr.GetControllerConfigRef() != nil {
		cc = &v1alpha1.ControllerConfig{}
		if err := h.client.Get(ctx, types.NamespacedName{Name: pr.GetControllerConfigRef().Name}, cc); err != nil {
			return nil, errors.Wrap(err, errControllerConfig)
		}
	}
	return cc, nil
}

// ConfigurationHooks performs operations for a configuration package before and
// after the revision establishes objects.
type ConfigurationHooks struct{}

// NewConfigurationHooks creates a new ConfigurationHook.
func NewConfigurationHooks() *ConfigurationHooks {
	return &ConfigurationHooks{}
}

// Pre sets status fields based on the configuration package.
func (h *ConfigurationHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1beta1.PackageRevision) error {
	_, ok := pkg.(*pkgmeta.Configuration)
	if !ok {
		return errors.New(errNotConfiguration)
	}

	// TODO(hasheddan): update any status fields relevant to package revisions

	return nil
}

// Post is a no op for configuration packages.
func (h *ConfigurationHooks) Post(context.Context, runtime.Object, v1beta1.PackageRevision) error {
	return nil
}

// NopHooks performs no operations.
type NopHooks struct{}

// NewNopHooks creates a hook that does nothing.
func NewNopHooks() *NopHooks {
	return &NopHooks{}
}

// Pre does nothing and returns nil.
func (h *NopHooks) Pre(context.Context, runtime.Object, v1beta1.PackageRevision) error {
	return nil
}

// Post does nothing and returns nil.
func (h *NopHooks) Post(context.Context, runtime.Object, v1beta1.PackageRevision) error {
	return nil
}

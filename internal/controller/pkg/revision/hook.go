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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errNotProvider                   = "not a provider package"
	errNotProviderRevision           = "not a provider revision"
	errGetControllerConfig           = "cannot get referenced controller config"
	errGetServiceAccount             = "cannot get Crossplane service account"
	errDeleteProviderDeployment      = "cannot delete provider package deployment"
	errDeleteProviderSA              = "cannot delete provider package service account"
	errDeleteProviderService         = "cannot delete provider package service"
	errDeleteProviderSecret          = "cannot delete provider package TLS secret"
	errApplyProviderDeployment       = "cannot apply provider package deployment"
	errApplyProviderSecret           = "cannot apply provider package secret"
	errApplyProviderSA               = "cannot apply provider package service account"
	errApplyProviderService          = "cannot apply provider package service"
	errUnavailableProviderDeployment = "provider package deployment is unavailable"

	errNotFunction                   = "not a function package"
	errDeleteFunctionDeployment      = "cannot delete function package deployment"
	errDeleteFunctionSA              = "cannot delete function package service account"
	errApplyFunctionDeployment       = "cannot apply function package deployment"
	errApplyFunctionSecret           = "cannot apply function package secret"
	errApplyFunctionSA               = "cannot apply function package service account"
	errApplyFunctionService          = "cannot apply function package service"
	errUnavailableFunctionDeployment = "function package deployment is unavailable"
)

// A Hooks performs operations before and after a revision establishes objects.
type Hooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1.PackageRevision) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1.PackageRevision) error
}

// ProviderHooks performs operations for a provider package that requires a
// controller before and after the revision establishes objects.
type ProviderHooks struct {
	client         resource.ClientApplicator
	namespace      string
	serviceAccount string
}

// NewProviderHooks creates a new ProviderHooks.
func NewProviderHooks(client resource.ClientApplicator, namespace, serviceAccount string) *ProviderHooks {
	return &ProviderHooks{
		client:         client,
		namespace:      namespace,
		serviceAccount: serviceAccount,
	}
}

// Pre cleans up a packaged controller and service account if the revision is
// inactive.
func (h *ProviderHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	pkgProvider, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New(errNotProvider)
	}

	provRev, ok := pr.(*v1.ProviderRevision)
	if !ok {
		return errors.New(errNotProviderRevision)
	}

	provRev.Status.PermissionRequests = pkgProvider.Spec.Controller.PermissionRequests

	// TODO(hasheddan): update any status fields relevant to package revisions.

	// Do not clean up SA and controller if revision is not inactive.
	if pr.GetDesiredState() != v1.PackageRevisionInactive {
		return nil
	}

	// NOTE(hasheddan): we avoid fetching pull secrets and controller config as
	// they aren't needed to delete Deployment, ServiceAccount, and Service.
	s, d, svc, secSer, secCli := buildProviderDeployment(pkgProvider, pr, nil, h.namespace, []corev1.LocalObjectReference{})
	if err := h.client.Delete(ctx, d); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}
	if err := h.client.Delete(ctx, s); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}
	if err := h.client.Delete(ctx, svc); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderService)
	}
	if err := h.client.Delete(ctx, secSer); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSecret)
	}
	if err := h.client.Delete(ctx, secCli); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSecret)
	}
	return nil
}

// Post creates a packaged provider controller and service account if the
// revision is active.
func (h *ProviderHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) error { //nolint:gocyclo // Only slightly over (12).
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	pkgProvider, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}
	cc, err := h.getControllerConfig(ctx, pr)
	if err != nil {
		return err
	}
	ps, err := h.getSAPullSecrets(ctx)
	if err != nil {
		return err
	}
	s, d, svc, secSer, secCli := buildProviderDeployment(pkgProvider, pr, cc, h.namespace, append(pr.GetPackagePullSecrets(), ps...))
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}
	owner := []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pkgProvider, pkgProvider.GetObjectKind().GroupVersionKind()))}
	if err := h.client.Apply(ctx, secSer); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}
	if err := h.client.Apply(ctx, secCli); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}
	if err := initializer.NewTLSCertificateGenerator(h.namespace, initializer.RootCACertSecretName, pkgProvider.Name,
		initializer.TLSCertificateGeneratorWithServerSecretName(pr.GetTLSServerSecretName()),
		initializer.TLSCertificateGeneratorWithClientSecretName(pr.GetTLSClientSecretName()),
		initializer.TLSCertificateGeneratorWithOwner(owner)).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %s", pkgProvider.Name)
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}
	if pr.GetWebhookTLSSecretName() != nil {
		if err := h.client.Apply(ctx, svc); err != nil {
			return errors.Wrap(err, errApplyProviderService)
		}
	}
	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf("%s: %s", errUnavailableProviderDeployment, c.Message)
		}
	}
	return nil
}

func (h *ProviderHooks) getSAPullSecrets(ctx context.Context) ([]corev1.LocalObjectReference, error) {
	sa := &corev1.ServiceAccount{}
	if err := h.client.Get(ctx, types.NamespacedName{
		Namespace: h.namespace,
		Name:      h.serviceAccount,
	}, sa); err != nil {
		return []corev1.LocalObjectReference{}, errors.Wrap(err, errGetServiceAccount)
	}
	return sa.ImagePullSecrets, nil
}

func (h *ProviderHooks) getControllerConfig(ctx context.Context, pr v1.PackageRevision) (*v1alpha1.ControllerConfig, error) {
	if pr.GetControllerConfigRef() == nil {
		return nil, nil
	}
	cc := &v1alpha1.ControllerConfig{}
	err := h.client.Get(ctx, types.NamespacedName{Name: pr.GetControllerConfigRef().Name}, cc)
	return cc, errors.Wrap(err, errGetControllerConfig)
}

// ConfigurationHooks performs operations for a configuration package before and
// after the revision establishes objects.
type ConfigurationHooks struct{}

// NewConfigurationHooks creates a new ConfigurationHook.
func NewConfigurationHooks() *ConfigurationHooks {
	return &ConfigurationHooks{}
}

// Pre sets status fields based on the configuration package.
func (h *ConfigurationHooks) Pre(_ context.Context, _ runtime.Object, _ v1.PackageRevision) error {
	return nil
}

// Post is a no op for configuration packages.
func (h *ConfigurationHooks) Post(context.Context, runtime.Object, v1.PackageRevision) error {
	return nil
}

// FunctionHooks performs operations for a function package that requires a
// controller before and after the revision establishes objects.
type FunctionHooks struct {
	client         resource.ClientApplicator
	namespace      string
	serviceAccount string
}

// NewFunctionHooks creates a new FunctionHooks.
func NewFunctionHooks(client resource.ClientApplicator, namespace, serviceAccount string) *FunctionHooks {
	return &FunctionHooks{
		client:         client,
		namespace:      namespace,
		serviceAccount: serviceAccount,
	}
}

// Pre cleans up a packaged controller and service account if the revision is
// inactive.
func (h *FunctionHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) error {
	fo, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	pkgFunction, ok := fo.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}

	// TODO(ezgidemirel): update any status fields relevant to package revisions.

	// Do not clean up SA and controller if revision is not inactive.
	if pr.GetDesiredState() != v1.PackageRevisionInactive {
		return nil
	}

	// NOTE(hasheddan): we avoid fetching pull secrets and controller config as
	// they aren't needed to delete Deployment and service account.
	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.
	s, d, _, _ := buildFunctionDeployment(pkgFunction, pr, nil, h.namespace, []corev1.LocalObjectReference{})
	if err := h.client.Delete(ctx, d); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteFunctionDeployment)
	}
	if err := h.client.Delete(ctx, s); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteFunctionSA)
	}

	return nil
}

// Post creates a packaged function deployment, service account, service and secrets if the revision is active.
func (h *FunctionHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) error { //nolint:gocyclo // See below
	// TODO(ezgidemirel): Can this be refactored for less complexity?
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	pkgFunction, ok := po.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}
	cc, err := h.getControllerConfig(ctx, pr)
	if err != nil {
		return err
	}
	ps, err := h.getSAPullSecrets(ctx)
	if err != nil {
		return err
	}
	s, d, svc, secSer := buildFunctionDeployment(pkgFunction, pr, cc, h.namespace, append(pr.GetPackagePullSecrets(), ps...))
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyFunctionSA)
	}
	owner := pr.GetOwnerReferences()
	if err := h.client.Apply(ctx, secSer); err != nil {
		return errors.Wrap(err, errApplyFunctionSecret)
	}
	if err := initializer.NewTLSCertificateGenerator(h.namespace, initializer.RootCACertSecretName, pkgFunction.Name,
		initializer.TLSCertificateGeneratorWithServerSecretName(pr.GetTLSServerSecretName()),
		initializer.TLSCertificateGeneratorWithOwner(owner)).GenerateServerCertificate(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %s", pkgFunction.Name)
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyFunctionDeployment)
	}
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyFunctionService)
	}

	fRev := pr.(*v1beta1.FunctionRevision)
	fRev.Status.Endpoint = fmt.Sprintf(serviceEndpointFmt, svc.Name, svc.Namespace, servicePort)

	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf("%s: %s", errUnavailableFunctionDeployment, c.Message)
		}
	}
	return nil
}

func (h *FunctionHooks) getSAPullSecrets(ctx context.Context) ([]corev1.LocalObjectReference, error) {
	sa := &corev1.ServiceAccount{}
	if err := h.client.Get(ctx, types.NamespacedName{
		Namespace: h.namespace,
		Name:      h.serviceAccount,
	}, sa); err != nil {
		return []corev1.LocalObjectReference{}, errors.Wrap(err, errGetServiceAccount)
	}
	return sa.ImagePullSecrets, nil
}

func (h *FunctionHooks) getControllerConfig(ctx context.Context, pr v1.PackageRevision) (*v1alpha1.ControllerConfig, error) {
	if pr.GetControllerConfigRef() == nil {
		return nil, nil
	}
	cc := &v1alpha1.ControllerConfig{}
	err := h.client.Get(ctx, types.NamespacedName{Name: pr.GetControllerConfigRef().Name}, cc)
	return cc, errors.Wrap(err, errGetControllerConfig)
}

// NopHooks performs no operations.
type NopHooks struct{}

// NewNopHooks creates a hook that does nothing.
func NewNopHooks() *NopHooks {
	return &NopHooks{}
}

// Pre does nothing and returns nil.
func (h *NopHooks) Pre(context.Context, runtime.Object, v1.PackageRevision) error {
	return nil
}

// Post does nothing and returns nil.
func (h *NopHooks) Post(context.Context, runtime.Object, v1.PackageRevision) error {
	return nil
}

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
	errNotProvider                            = "not a provider package"
	errNotProviderRevision                    = "not a provider revision"
	errGetControllerConfig                    = "cannot get referenced controller config"
	errGetServiceAccount                      = "cannot get Crossplane service account"
	errDeleteProviderDeployment               = "cannot delete provider package deployment"
	errDeleteProviderSA                       = "cannot delete provider package service account"
	errDeleteProviderService                  = "cannot delete provider package service"
	errApplyProviderDeployment                = "cannot apply provider package deployment"
	errApplyProviderSecret                    = "cannot apply provider package secret"
	errApplyProviderSA                        = "cannot apply provider package service account"
	errApplyProviderService                   = "cannot apply provider package service"
	errFmtUnavailableProviderDeployment       = "provider package deployment is unavailable with message: %s"
	errNoAvailableConditionProviderDeployment = "provider package deployment has no condition of type \"Available\" yet"

	errNotFunction                            = "not a function package"
	errDeleteFunctionDeployment               = "cannot delete function package deployment"
	errDeleteFunctionSA                       = "cannot delete function package service account"
	errApplyFunctionDeployment                = "cannot apply function package deployment"
	errApplyFunctionSecret                    = "cannot apply function package secret"
	errApplyFunctionSA                        = "cannot apply function package service account"
	errApplyFunctionService                   = "cannot apply function package service"
	errFmtUnavailableFunctionDeployment       = "function package deployment is unavailable with message: %s"
	errNoAvailableConditionFunctionDeployment = "function package deployment has no condition of type \"Available\" yet"
)

// A Hooks performs operations before and after a revision establishes objects.
type Hooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1.PackageRevision) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1.PackageRevision) error

	// Deactivate performs operations meant to happen before deactivating a revision.
	Deactivate(context.Context, v1.PackageRevision) error
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

// Pre fills permission requests from the provider package to the revision.
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

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	return h.ensurePrerequisites(ctx, pkgProvider, pr)
}

// ensurePrerequisites ensures that the required secrets and services are
// created for the provider package revision
func (h *ProviderHooks) ensurePrerequisites(ctx context.Context, pkgProvider *pkgmetav1.Provider, pr v1.PackageRevision) error {
	svc := buildProviderService(pkgProvider, pr, h.namespace)
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyProviderService)
	}

	secSer, secCli := buildProviderSecrets(pr, h.namespace)

	if secSer == nil || secCli == nil {
		// we should wait for the provider revision reconciler to set the secret names before proceeding creating the TLS secrets
		return nil
	}
	if err := h.client.Apply(ctx, secSer); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}
	if err := h.client.Apply(ctx, secCli); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}

	if err := initializer.NewTLSCertificateGenerator(h.namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithOwner(pr.GetOwnerReferences()),
		initializer.TLSCertificateGeneratorWithServerSecretName(secSer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithClientSecretName(secCli.GetName(), []string{pr.GetName()})).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %s", pr.GetName())
	}
	return nil
}

// Post creates a packaged provider controller and service account if the
// revision is active.
func (h *ProviderHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) error {
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

	s, d := buildProviderDeployment(pkgProvider, pr, cc, h.namespace, append(pr.GetPackagePullSecrets(), ps...))
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}
	// TODO(phisco): check who is actually using this
	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf(errFmtUnavailableProviderDeployment, c.Message)
		}
	}
	return errors.New(errNoAvailableConditionProviderDeployment)
}

// Deactivate performs operations meant to happen before deactivating a provider revision.
func (h *ProviderHooks) Deactivate(ctx context.Context, pr v1.PackageRevision) error {
	// Delete the deployment if it exists.
	if err := h.client.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: pr.GetName(), Namespace: h.namespace}}); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: pr.GetName(), Namespace: h.namespace}}); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// TODO(phisco): only added to cleanup the service we were previously
	// 	deploying for each provider revision, remove in a future release.
	if err := h.client.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: pr.GetName(), Namespace: h.namespace}}); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderService)
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

// Deactivate is a no op for configuration packages.
func (h *ConfigurationHooks) Deactivate(_ context.Context, _ v1.PackageRevision) error {
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

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	return h.ensurePrerequisites(ctx, pkgFunction, pr)
}

// ensurePrerequisites ensures that the required secrets and services are
// created for the function package revision.
func (h *FunctionHooks) ensurePrerequisites(ctx context.Context, pkgFunction *pkgmetav1beta1.Function, pr v1.PackageRevision) error {
	svc := buildFunctionService(pkgFunction, pr, h.namespace)
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyFunctionService)
	}

	// N.B.: We expect the revision to be applied by the caller
	fRev := pr.(*v1beta1.FunctionRevision)
	fRev.Status.Endpoint = fmt.Sprintf(serviceEndpointFmt, svc.Name, svc.Namespace, servicePort)

	secSer := buildFunctionSecret(pr, h.namespace)
	if err := h.client.Apply(ctx, secSer); err != nil {
		return errors.Wrap(err, errApplyFunctionSecret)
	}
	if err := initializer.NewTLSCertificateGenerator(h.namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithServerSecretName(secSer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithOwner([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))})).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %s", pkgFunction.Name)
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

	s, d := buildFunctionDeployment(pkgFunction, pr, cc, h.namespace, append(pr.GetPackagePullSecrets(), ps...))
	if err := h.client.Apply(ctx, s); err != nil {
		return errors.Wrap(err, errApplyFunctionSA)
	}

	svc := buildFunctionService(pkgFunction, pr, h.namespace)

	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyFunctionDeployment)
	}

	fRev := pr.(*v1beta1.FunctionRevision)
	fRev.Status.Endpoint = fmt.Sprintf(serviceEndpointFmt, svc.Name, svc.Namespace, servicePort)

	// TODO(phisco): check who is actually using this
	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf(errFmtUnavailableFunctionDeployment, c.Message)
		}
	}
	return errors.New(errNoAvailableConditionFunctionDeployment)
}

// Deactivate performs operations meant to happen for deactivating a function revision.
func (h *FunctionHooks) Deactivate(ctx context.Context, pr v1.PackageRevision) error {
	// Delete the deployment if it exists.
	if err := h.client.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: pr.GetName(), Namespace: h.namespace}}); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteFunctionDeployment)
	}
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: pr.GetName(), Namespace: h.namespace}}); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteFunctionSA)
	}

	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.
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

// Deactivate does nothing and returns nil.
func (h *NopHooks) Deactivate(context.Context, v1.PackageRevision) error {
	return nil
}

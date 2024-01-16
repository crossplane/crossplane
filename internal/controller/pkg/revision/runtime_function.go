/*
Copyright 2023 The Crossplane Authors.

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

	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errNotFunction                            = "not a function package"
	errDeleteFunctionDeployment               = "cannot delete function package deployment"
	errDeleteFunctionSA                       = "cannot delete function package service account"
	errApplyFunctionDeployment                = "cannot apply function package deployment"
	errApplyFunctionSecret                    = "cannot apply function package secret"
	errApplyFunctionSA                        = "cannot apply function package service account"
	errApplyFunctionService                   = "cannot apply function package service"
	errFmtUnavailableFunctionDeployment       = "function package deployment is unavailable with message: %s"
	errNoAvailableConditionFunctionDeployment = "function package deployment has no condition of type \"Available\" yet"
	errParseFunctionImage                     = "cannot parse function package image"
)

// FunctionHooks performs runtime operations for function packages.
type FunctionHooks struct {
	client          resource.ClientApplicator
	defaultRegistry string
}

// NewFunctionHooks returns a new FunctionHooks.
func NewFunctionHooks(client client.Client, defaultRegistry string) *FunctionHooks {
	return &FunctionHooks{
		client: resource.ClientApplicator{
			Client:     client,
			Applicator: resource.NewAPIPatchingApplicator(client),
		},
		defaultRegistry: defaultRegistry,
	}
}

// Pre performs operations meant to happen before establishing objects.
func (h *FunctionHooks) Pre(ctx context.Context, _ runtime.Object, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	// TODO(ezgidemirel): update any status fields relevant to package revisions.

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Ensure Prerequisites
	// Note(turkenh): We need certificates have generated when we get to the
	// establish step, i.e. we want to inject the CA to CRDs (webhook caBundle).
	// Therefore, we need to generate the certificates pre establish and
	// generating certificates requires the service to be defined. This is why
	// we're creating the service here but service account and deployment in the
	// post establish.
	// As a rule of thumb, we create objects named after the package in the
	// pre hook and objects named after the package revision in the post hook.
	svc := build.Service(functionServiceOverrides()...)
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyFunctionService)
	}

	// N.B.: We expect the revision to be applied by the caller
	fRev := pr.(*v1beta1.FunctionRevision)
	fRev.Status.Endpoint = fmt.Sprintf(serviceEndpointFmt, svc.Name, svc.Namespace, servicePort)

	secServer := build.TLSServerSecret()
	if err := h.client.Apply(ctx, secServer); err != nil {
		return errors.Wrap(err, errApplyFunctionSecret)
	}

	if err := initializer.NewTLSCertificateGenerator(secServer.Namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithOwner([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))})).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	return nil
}

// Post performs operations meant to happen after establishing objects.
func (h *FunctionHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	functionMeta, ok := po.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := build.ServiceAccount()

	// Determine the function's image, taking into account the default registry.
	image, err := getFunctionImage(functionMeta, pr, h.defaultRegistry)
	if err != nil {
		return errors.Wrap(err, errParseFunctionImage)
	}

	d := build.Deployment(sa.Name, functionDeploymentOverrides(image)...)
	// Create/Apply the SA only if the deployment references it.
	// This is to avoid creating a SA that is NOT used by the deployment when
	// the SA is managed externally by the user and configured by setting
	// `deploymentTemplate.spec.template.spec.serviceAccountName` in the
	// DeploymentRuntimeConfig.
	if sa.Name == d.Spec.Template.Spec.ServiceAccountName {
		if err := h.client.Apply(ctx, sa); err != nil {
			return errors.Wrap(err, errApplyFunctionSA)
		}
	}
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyFunctionDeployment)
	}

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

// Deactivate performs operations meant to happen before deactivating a revision.
func (h *FunctionHooks) Deactivate(ctx context.Context, _ v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	sa := build.ServiceAccount()
	// Delete the deployment if it exists.
	// Different from the Post runtimeHook, we don't need to pass the
	// "functionDeploymentOverrides()" here, because we're only interested
	// in the name and namespace of the deployment to delete it.
	if err := h.client.Delete(ctx, build.Deployment(sa.Name)); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteFunctionDeployment)
	}

	// NOTE(turkenh): We don't delete the service account here because it might
	// be used by other package revisions, e.g. user might have specified a
	// service account name in the runtime config. This should not be a problem
	// because we add the owner reference to the service account when we create
	// them, and they will be garbage collected when the package revision is
	// deleted if they are not used by any other package revisions.

	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.
	return nil
}

func functionDeploymentOverrides(image string) []DeploymentOverride {
	do := []DeploymentOverride{
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          grpcPortName,
				ContainerPort: servicePort,
			},
		}),
	}

	do = append(do, DeploymentRuntimeWithOptionalImage(image))

	return do
}

func functionServiceOverrides() []ServiceOverride {
	return []ServiceOverride{
		// We want a headless service so that our gRPC client (i.e. the Crossplane
		// FunctionComposer) can load balance across the endpoints.
		// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
		ServiceWithClusterIP(corev1.ClusterIPNone),
	}
}

// getFunctionImage determines a complete function image, taking into account a
// default registry. If the function meta specifies an image, we have a
// preference for that image over what is specified in the package revision.
func getFunctionImage(fm *pkgmetav1beta1.Function, pr v1.PackageRevisionWithRuntime, defaultRegistry string) (string, error) {
	image := pr.GetSource()
	if fm.Spec.Image != nil {
		image = *fm.Spec.Image
	}
	ref, err := name.ParseReference(image, name.WithDefaultRegistry(defaultRegistry))
	if err != nil {
		return "", errors.Wrap(err, errParseFunctionImage)
	}

	return ref.Name(), nil
}

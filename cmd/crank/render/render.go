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

package render

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Annotations added to composed resources.
const (
	AnnotationKeyCompositionResourceName = "crossplane.io/composition-resource-name"
	AnnotationKeyCompositeName           = "crossplane.io/composite"
	AnnotationKeyClaimNamespace          = "crossplane.io/claim-namespace"
	AnnotationKeyClaimName               = "crossplane.io/claim-name"
)

// CompositionInputs contains all inputs to the render process.
type CompositionInputs struct {
	CompositeResource   *ucomposite.Unstructured
	Composition         *apiextensionsv1.Composition
	FunctionAddrs       map[string]string
	FunctionCredentials []corev1.Secret
	ObservedResources   []composed.Unstructured
	RequiredResources   []kunstructured.Unstructured
	RequiredSchemas     []spec3.OpenAPI
}

// CompositionOutputs contains all outputs from the render process.
type CompositionOutputs struct {
	CompositeResource *ucomposite.Unstructured
	ComposedResources []composed.Unstructured
	Results           []kunstructured.Unstructured
	Context           *kunstructured.Unstructured
}

// OperationInputs contains all inputs to the render process for an operation.
type OperationInputs struct {
	Operation           *opsv1alpha1.Operation
	FunctionAddrs       map[string]string
	FunctionCredentials []corev1.Secret
	RequiredResources   []kunstructured.Unstructured
	RequiredSchemas     []spec3.OpenAPI
}

// OperationOutputs contains all outputs from the render process.
type OperationOutputs struct {
	Operation        *opsv1alpha1.Operation
	AppliedResources []kunstructured.Unstructured
	Results          []kunstructured.Unstructured
	Context          *kunstructured.Unstructured
}

// FunctionAddresses maps function names to their gRPC target addresses.
type FunctionAddresses struct {
	addrs    map[string]string
	contexts map[string]RuntimeContext
}

// Addresses returns the function name to gRPC address map.
func (fa *FunctionAddresses) Addresses() map[string]string {
	return fa.addrs
}

// Stop all function runtimes.
func (fa *FunctionAddresses) Stop(ctx context.Context) error {
	for name, rctx := range fa.contexts {
		if err := rctx.Stop(ctx); err != nil {
			return errors.Wrapf(err, "cannot stop function %q runtime (target %q)", name, rctx.Target)
		}
	}
	return nil
}

// StartFunctionRuntimes starts the runtime for each function and returns their
// gRPC addresses. The caller must call Stop on the returned FunctionAddresses
// when done.
func StartFunctionRuntimes(ctx context.Context, log logging.Logger, fns []pkgv1.Function) (*FunctionAddresses, error) {
	addrs := make(map[string]string, len(fns))
	contexts := make(map[string]RuntimeContext, len(fns))

	for _, fn := range fns {
		rt, err := GetRuntime(fn, log)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get runtime for Function %q", fn.GetName())
		}

		rctx, err := rt.Start(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot start Function %q", fn.GetName())
		}

		addrs[fn.GetName()] = rctx.Target
		contexts[fn.GetName()] = rctx
	}

	return &FunctionAddresses{addrs: addrs, contexts: contexts}, nil
}

// RewriteAddressesForDocker rewrites function addresses so they are reachable
// from inside a Docker container. Addresses targeting localhost or 127.0.0.1
// are rewritten to host.docker.internal.
func RewriteAddressesForDocker(fns []*renderv1alpha1.FunctionInput) []*renderv1alpha1.FunctionInput {
	for _, fn := range fns {
		fn.Address = strings.Replace(fn.GetAddress(), "localhost:", "host.docker.internal:", 1)
		fn.Address = strings.Replace(fn.GetAddress(), "127.0.0.1:", "host.docker.internal:", 1)
	}
	return fns
}

// GetSecret retrieves the secret with the specified name and namespace from the provided list of secrets.
func GetSecret(name string, nameSpace string, secrets []corev1.Secret) (*corev1.Secret, error) {
	for _, s := range secrets {
		if s.GetName() == name && s.GetNamespace() == nameSpace {
			return &s, nil
		}
	}

	return nil, errors.Errorf("secret %q not found", name)
}

// SetComposedResourceMetadata sets standard, required composed resource
// metadata. It mirrors the behavior of RenderComposedResourceMetadata in
// Crossplane's composition controller.
func SetComposedResourceMetadata(cd resource.Object, xr resource.LegacyComposite, name string) error {
	namePrefix := xr.GetLabels()[AnnotationKeyCompositeName]
	if namePrefix == "" {
		namePrefix = xr.GetName()
	}

	if cd.GetName() == "" && cd.GetGenerateName() == "" {
		cd.SetGenerateName(namePrefix + "-")
	}

	if xr.GetNamespace() != "" {
		cd.SetNamespace(xr.GetNamespace())
	}

	meta.AddAnnotations(cd, map[string]string{AnnotationKeyCompositionResourceName: name})
	meta.AddLabels(cd, map[string]string{AnnotationKeyCompositeName: namePrefix})

	if xr.GetLabels()[AnnotationKeyClaimName] != "" && xr.GetLabels()[AnnotationKeyClaimNamespace] != "" {
		meta.AddLabels(cd, map[string]string{
			AnnotationKeyClaimNamespace: xr.GetLabels()[AnnotationKeyClaimNamespace],
			AnnotationKeyClaimName:      xr.GetLabels()[AnnotationKeyClaimName],
		})
	} else if ref := xr.GetClaimReference(); ref != nil {
		meta.AddLabels(cd, map[string]string{
			AnnotationKeyClaimNamespace: ref.Namespace,
			AnnotationKeyClaimName:      ref.Name,
		})
	}

	or := meta.AsController(meta.TypedReferenceTo(xr, xr.GetObjectKind().GroupVersionKind()))

	return errors.Wrapf(meta.AddControllerReference(cd, or), "cannot set composite resource %q as controller ref of composed resource", xr.GetName())
}

// injectNetworkAnnotation sets the Docker network annotation on all functions
// so their containers join the specified network.
func injectNetworkAnnotation(fns []pkgv1.Function, networkName string) {
	for i := range fns {
		if fns[i].Annotations == nil {
			fns[i].Annotations = make(map[string]string)
		}
		fns[i].Annotations[AnnotationKeyRuntimeDockerNetwork] = networkName
	}
}

// StopFunctionRuntimes stops all function runtimes with a timeout.
func StopFunctionRuntimes(log logging.Logger, fa *FunctionAddresses) {
	if fa == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := fa.Stop(stopCtx); err != nil {
		log.Info("Error stopping function runtimes", "error", err)
	}
}

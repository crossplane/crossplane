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

// Package composite renders a composite resource (XR) by running one real
// reconcile loop against a fake in-memory client. It is intended as an
// internal engine for tools like 'crossplane render' and 'up test run'.
package composite

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	managed "github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	apis "github.com/crossplane/crossplane/apis/v2"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/internal/circuit"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composition"
	"github.com/crossplane/crossplane/v2/internal/render"
	"github.com/crossplane/crossplane/v2/internal/ssa"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

// Render runs one real XR reconcile loop using the real reconciler engine
// backed by a fake in-memory client.
func Render(ctx context.Context, log logging.Logger, in *Input) (*Output, error) {
	// Build the scheme for the fake client.
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add core/v1 to scheme")
	}
	if err := apis.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add Crossplane APIs to scheme")
	}

	// Wrap the input XR in the composite.Unstructured type.
	xr := ucomposite.New()
	xr.Object = in.CompositeResource.Object
	gvk := xr.GroupVersionKind()

	// Set a deterministic fake UID. The NameGenerator uses the owner UID for
	// deterministic name generation of composed resources.
	xr.SetUID(types.UID(uuid.NewSHA1(uuid.Nil, []byte(gvk.String()+xr.GetName())).String()))

	// Set a resourceVersion to avoid "object has no resource version" errors.
	xr.SetResourceVersion("999")

	// Inject spec.resourceRefs for observed resources so the real
	// ExistingComposedResourceObserver can find them via client.Get.
	injectResourceRefs(xr, in.ObservedResources)

	// Synthesize a CompositionRevision from the Composition.
	rev := composition.NewCompositionRevision(&in.Composition, 1)
	rev.Status.SetConditions(apiextensionsv1.ValidPipeline())

	// Build the in-memory store with all input resources.
	storeResources, err := buildStoreResources(xr, rev, in)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build store resources")
	}
	fakeClient := render.NewInMemoryClient(s, storeResources...)

	// Connect to function gRPC servers. The caller is responsible for
	// starting function runtimes.
	runner, err := render.NewFunctionRunner(in.Functions)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to functions")
	}
	defer runner.Close() //nolint:errcheck // Best-effort cleanup.

	// Build the fetching runner for the requirements protocol.
	fetcher := xfn.NewExistingRequiredResourcesFetcher(fakeClient)
	fetchingRunner := xfn.NewFetchingFunctionRunner(runner, fetcher, xfn.NopRequiredSchemasFetcher{})

	// Build the initial function context from input.
	initialCtx, err := buildInitialContext(in.Context)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build initial function context")
	}

	// Build the real FunctionComposer with real implementations backed by
	// the fake client.
	fc := composite.NewFunctionComposer(fakeClient, fakeClient, fetchingRunner,
		composite.WithComposedResourceObserver(
			composite.NewExistingComposedResourceObserver(fakeClient, fakeClient,
				composite.NewSecretConnectionDetailsFetcher(fakeClient))),
		composite.WithComposedResourceGarbageCollector(
			composite.NewDeletingComposedResourceGarbageCollector(fakeClient)),
		composite.WithCompositeConnectionDetailsFetcher(
			composite.NewSecretConnectionDetailsFetcher(fakeClient)),
		composite.WithManagedFieldsUpgrader(&ssa.NopManagedFieldsUpgrader{}),
		composite.WithInitialContext(initialCtx),
	)

	// Build a recording event recorder to capture events for output.
	recorder := &render.EventRecorder{}

	// Build the real Reconciler with production logic and fake I/O
	// dependencies.
	r := composite.NewReconciler(fakeClient, gvk,
		composite.WithComposer(fc),
		composite.WithCompositeFinalizer(xpresource.NewNopFinalizer()),
		composite.WithCompositionSelector(CompositionSelector(&in.Composition)),
		composite.WithCompositionRevisionSelector(composite.CompositionRevisionSelectorFn(
			func(_ context.Context, _ xpresource.Composite) error { return nil },
		)),
		composite.WithCompositionRevisionFetcher(composite.CompositionRevisionFetcherFn(
			func(_ context.Context, _ xpresource.Composite) (*apiextensionsv1.CompositionRevision, error) {
				return rev, nil
			},
		)),
		composite.WithConfigurator(composite.NewConfiguratorChain(
			composite.NewAPINamingConfigurator(fakeClient),
			composite.NewAPIConfigurator(fakeClient),
		)),
		composite.WithConnectionPublishers(composite.ConnectionPublisherFn(
			func(_ context.Context, _ composite.ConnectionSecretOwner, _ managed.ConnectionDetails) (bool, error) {
				return false, nil
			},
		)),
		composite.WithWatchStarter("render", nil, &composite.NopWatchStarter{}),
		composite.WithCircuitBreaker(&circuit.NopBreaker{}),
		composite.WithRecorder(recorder),
		composite.WithLogger(log),
	)

	// Run one reconcile loop.
	req := reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: xr.GetNamespace(),
		Name:      xr.GetName(),
	}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		return nil, errors.Wrap(err, "reconcile failed")
	}

	return BuildOutput(fakeClient, gvk, recorder), nil
}

// injectResourceRefs adds spec.resourceRefs to the XR for each observed
// resource. The real ExistingComposedResourceObserver reads these refs to
// discover existing composed resources.
func injectResourceRefs(xr *ucomposite.Unstructured, observed []kunstructured.Unstructured) {
	if len(observed) == 0 {
		return
	}

	refs := xr.GetResourceReferences()
	for _, o := range observed {
		refs = append(refs, corev1.ObjectReference{
			APIVersion: o.GetAPIVersion(),
			Kind:       o.GetKind(),
			Name:       o.GetName(),
			Namespace:  o.GetNamespace(),
		})
	}

	xr.SetResourceReferences(refs)
}

// buildStoreResources assembles all resources to load into the fake client's
// in-memory store.
func buildStoreResources(xr *ucomposite.Unstructured, rev *apiextensionsv1.CompositionRevision, in *Input) ([]kunstructured.Unstructured, error) {
	resources := make([]kunstructured.Unstructured, 0,
		2+len(in.ObservedResources)+len(in.RequiredResources)+len(in.ExtraResources)+len(in.Credentials))

	// The XR itself.
	resources = append(resources, xr.Unstructured)

	// The synthesized CompositionRevision.
	revData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rev)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert CompositionRevision to unstructured")
	}
	u := kunstructured.Unstructured{Object: revData}
	u.SetGroupVersionKind(apiextensionsv1.CompositionRevisionGroupVersionKind)
	resources = append(resources, u)

	// Observed composed resources.
	resources = append(resources, in.ObservedResources...)

	// Required resources.
	resources = append(resources, in.RequiredResources...)

	// Extra resources.
	resources = append(resources, in.ExtraResources...)

	// Credential secrets.
	for i := range in.Credentials {
		secretData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&in.Credentials[i])
		if err != nil {
			return nil, errors.Wrapf(err, "cannot convert credential secret %q to unstructured", in.Credentials[i].GetName())
		}
		su := kunstructured.Unstructured{Object: secretData}
		su.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
		resources = append(resources, su)
	}

	return resources, nil
}

// buildInitialContext converts the input context map to a structpb.Struct for
// seeding the function pipeline.
func buildInitialContext(inputCtx map[string]runtime.RawExtension) (*structpb.Struct, error) {
	if len(inputCtx) == 0 {
		return nil, nil
	}

	fctx := &structpb.Struct{Fields: make(map[string]*structpb.Value, len(inputCtx))}
	for k, raw := range inputCtx {
		var jv any
		if err := json.Unmarshal(raw.Raw, &jv); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal context key %q", k)
		}

		v, err := structpb.NewValue(jv)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot convert context key %q to protobuf value", k)
		}

		fctx.Fields[k] = v
	}

	return fctx, nil
}

// CompositionSelector returns a CompositionSelectorFn that sets the
// composition reference on the XR to point to the supplied Composition.
func CompositionSelector(comp *apiextensionsv1.Composition) composite.CompositionSelectorFn {
	return func(_ context.Context, cr xpresource.Composite) error {
		cr.SetCompositionReference(&corev1.ObjectReference{
			Name: comp.GetName(),
		})
		return nil
	}
}

// BuildOutput assembles an Output from the fake client's captured state and
// the event recorder.
func BuildOutput(c *render.InMemoryClient, xrGVK schema.GroupVersionKind, recorder *render.EventRecorder) *Output {
	out := &Output{
		APIVersion:        APIVersion,
		Kind:              KindOutput,
		ComposedResources: []kunstructured.Unstructured{},
	}

	// Find the final XR state. It's the last Status().Update or
	// Status().Patch call for the XR's GVK.
	for i := len(c.Updated()) - 1; i >= 0; i-- {
		u := c.Updated()[i]
		if u.GroupVersionKind() == xrGVK {
			out.CompositeResource = u
			break
		}
	}

	// If the XR wasn't in the updated list, check applied (from
	// Status().Patch, which records to applied).
	if out.CompositeResource.Object == nil {
		for i := len(c.Applied()) - 1; i >= 0; i-- {
			u := c.Applied()[i]
			if u.GroupVersionKind() == xrGVK {
				out.CompositeResource = u
				break
			}
		}
	}

	// Collect composed resources from applied (SSA Patch). Exclude the XR
	// itself and the XR resourceRefs patch (which has the XR's GVK).
	for _, u := range c.Applied() {
		if u.GroupVersionKind() == xrGVK {
			continue
		}
		out.ComposedResources = append(out.ComposedResources, u)
	}

	// Collect deleted resources.
	out.DeletedResources = c.Deleted()

	// Collect events.
	out.Events = append(out.Events, recorder.Events()...)

	return out
}

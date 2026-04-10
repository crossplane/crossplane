/*
Copyright 2026 The Crossplane Authors.

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

	"github.com/google/uuid"
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
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Render runs one real XR reconcile loop using the real reconciler engine
// backed by a fake in-memory client.
func Render(ctx context.Context, log logging.Logger, in *renderv1alpha1.CompositeInput) (*renderv1alpha1.CompositeOutput, error) {
	// Build the scheme for the fake client.
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add core/v1 to scheme")
	}
	if err := apis.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add Crossplane APIs to scheme")
	}

	// Convert the input XR from protobuf Struct to unstructured.
	xr := ucomposite.New()
	if err := xfn.FromStruct(xr, in.GetCompositeResource()); err != nil {
		return nil, errors.Wrap(err, "cannot convert composite resource from protobuf")
	}
	gvk := xr.GroupVersionKind()

	// Set a deterministic fake UID. The NameGenerator uses the owner UID for
	// deterministic name generation of composed resources.
	xr.SetUID(types.UID(uuid.NewSHA1(uuid.Nil, []byte(gvk.String()+xr.GetName())).String()))

	// Set a resourceVersion to avoid "object has no resource version" errors.
	xr.SetResourceVersion("999")

	// Convert observed resources from protobuf.
	observed := make([]kunstructured.Unstructured, 0, len(in.GetObservedResources()))
	for _, s := range in.GetObservedResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert observed resource from protobuf")
		}
		observed = append(observed, *u)
	}

	// Inject spec.resourceRefs for observed resources so the real
	// ExistingComposedResourceObserver can find them via client.Get.
	InjectResourceRefs(xr, observed)

	// Convert the Composition from protobuf.
	comp := &apiextensionsv1.Composition{}
	if err := xfn.FromStruct(comp, in.GetComposition()); err != nil {
		return nil, errors.Wrap(err, "cannot convert composition from protobuf")
	}

	// Synthesize a CompositionRevision from the Composition.
	rev := composition.NewCompositionRevision(comp, 1)
	rev.Status.SetConditions(apiextensionsv1.ValidPipeline())

	// Build the in-memory store with all input resources.
	storeResources := []kunstructured.Unstructured{xr.Unstructured}

	revData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rev)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert CompositionRevision to unstructured")
	}
	revU := kunstructured.Unstructured{Object: revData}
	revU.SetGroupVersionKind(apiextensionsv1.CompositionRevisionGroupVersionKind)
	storeResources = append(storeResources, revU)

	storeResources = append(storeResources, observed...)

	for _, s := range in.GetRequiredResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert required resource from protobuf")
		}
		storeResources = append(storeResources, *u)
	}

	for _, s := range in.GetCredentials() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert credential from protobuf")
		}
		if u.GetKind() == "" {
			u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
		}
		storeResources = append(storeResources, *u)
	}

	fakeClient := render.NewInMemoryClient(s, storeResources...)

	// Connect to function gRPC servers. The caller is responsible for
	// starting function runtimes.
	runner, err := render.NewFunctionRunner(in.GetFunctions())
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to functions")
	}
	defer runner.Close() //nolint:errcheck // Best-effort cleanup.

	// Build the fetching runner for the requirements protocol.
	fetcher := xfn.NewExistingRequiredResourcesFetcher(fakeClient)
	fetchingRunner := xfn.NewFetchingFunctionRunner(runner, fetcher, xfn.NopRequiredSchemasFetcher{})

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
	)

	// Build a recording event recorder to capture events for output.
	recorder := &render.EventRecorder{}

	// Build the real Reconciler with production logic and fake I/O
	// dependencies.
	r := composite.NewReconciler(fakeClient, gvk,
		composite.WithComposer(fc),
		composite.WithCompositeFinalizer(xpresource.NewNopFinalizer()),
		composite.WithCompositionSelector(CompositionSelector(comp)),
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

	isXR := func(u kunstructured.Unstructured) bool {
		return u.GroupVersionKind() == gvk &&
			u.GetNamespace() == xr.GetNamespace() &&
			u.GetName() == xr.GetName()
	}
	return BuildOutput(fakeClient, isXR, recorder)
}

// InjectResourceRefs sets spec.resourceRefs on the XR for each observed
// resource. The real ExistingComposedResourceObserver reads these refs to
// discover existing composed resources. Any preexisting refs are replaced.
func InjectResourceRefs(xr *ucomposite.Unstructured, observed []kunstructured.Unstructured) {
	if len(observed) == 0 {
		return
	}

	refs := make([]corev1.ObjectReference, 0, len(observed))
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

// BuildOutput assembles a CompositeOutput from the fake client's captured
// state and the event recorder. The isPrimary predicate identifies the
// primary resource (the XR) so it can be separated from composed resources.
func BuildOutput(c *render.InMemoryClient, isPrimary func(kunstructured.Unstructured) bool, recorder *render.EventRecorder) (*renderv1alpha1.CompositeOutput, error) {
	out := &renderv1alpha1.CompositeOutput{}

	// Find the final XR state. It's the last Status().Update or
	// Status().Patch call for the XR.
	for i := len(c.Updated()) - 1; i >= 0; i-- {
		u := c.Updated()[i]
		if isPrimary(u) {
			s, err := xfn.AsStruct(&u)
			if err != nil {
				return nil, errors.Wrap(err, "cannot convert composite resource to protobuf")
			}
			out.CompositeResource = s
			break
		}
	}

	// If the XR wasn't in the updated list, check applied (from
	// Status().Patch, which records to applied).
	if out.GetCompositeResource() == nil {
		for i := len(c.Applied()) - 1; i >= 0; i-- {
			u := c.Applied()[i]
			if isPrimary(u) {
				s, err := xfn.AsStruct(&u)
				if err != nil {
					return nil, errors.Wrap(err, "cannot convert composite resource to protobuf")
				}
				out.CompositeResource = s
				break
			}
		}
	}

	// Collect composed resources from applied (SSA Patch). Exclude the XR
	// itself and the XR resourceRefs patch.
	for _, u := range c.Applied() {
		if isPrimary(u) {
			continue
		}
		s, err := xfn.AsStruct(&u)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert composed resource to protobuf")
		}
		out.ComposedResources = append(out.ComposedResources, s)
	}

	// Collect deleted resources.
	for _, u := range c.Deleted() {
		s, err := xfn.AsStruct(&u)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert deleted resource to protobuf")
		}
		out.DeletedResources = append(out.DeletedResources, s)
	}

	// Collect events.
	out.Events = append(out.Events, recorder.Events()...)

	return out, nil
}

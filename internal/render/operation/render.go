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

// Package operation renders an Operation by running one real reconcile loop
// against a fake in-memory client.
package operation

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	apis "github.com/crossplane/crossplane/apis/v2"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	oprec "github.com/crossplane/crossplane/v2/internal/controller/ops/operation"
	"github.com/crossplane/crossplane/v2/internal/render"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

// Render runs one real Operation reconcile loop using the real reconciler
// engine backed by a fake in-memory client.
func Render(ctx context.Context, log logging.Logger, in *Input) (*Output, error) {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add core/v1 to scheme")
	}
	if err := apis.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add Crossplane APIs to scheme")
	}

	// Convert the Operation to unstructured for the fake client store.
	opData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&in.Operation)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation to unstructured")
	}
	opUnstructured := kunstructured.Unstructured{Object: opData}
	opUnstructured.SetGroupVersionKind(opsv1alpha1.OperationGroupVersionKind)

	// Set a resourceVersion to avoid errors from client operations.
	opUnstructured.SetResourceVersion("999")

	// Build the in-memory store with all input resources.
	storeResources, err := buildStoreResources(&opUnstructured, in)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build store resources")
	}
	fakeClient := render.NewInMemoryClient(s, storeResources...)

	// Connect to function gRPC servers.
	runner, err := render.NewFunctionRunner(in.Functions)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to functions")
	}
	defer runner.Close() //nolint:errcheck // Best-effort cleanup.

	// Build the fetching runner for the requirements protocol.
	fetcher := xfn.NewExistingRequiredResourcesFetcher(fakeClient)
	fetchingRunner := xfn.NewFetchingFunctionRunner(runner, fetcher, xfn.NopRequiredSchemasFetcher{})

	// Build the recording event recorder.
	recorder := &render.EventRecorder{}

	// Build the real Operation Reconciler with the fake client and
	// injected dependencies.
	r := oprec.NewReconciler(nil,
		oprec.WithClient(fakeClient),
		oprec.WithFunctionRunner(fetchingRunner),
		oprec.WithRequiredResourcesFetcher(fetcher),
		oprec.WithCapabilityChecker(xfn.CapabilityCheckerFn(
			func(_ context.Context, _ []string, _ ...string) error {
				// In render mode, we trust the caller provided valid functions.
				return nil
			},
		)),
		oprec.WithRecorder(recorder),
		oprec.WithLogger(log),
	)

	// Run one reconcile loop.
	req := reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: in.Operation.GetNamespace(),
		Name:      in.Operation.GetName(),
	}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		return nil, errors.Wrap(err, "reconcile failed")
	}

	return buildOutput(fakeClient, recorder), nil
}

// buildStoreResources assembles all resources to load into the fake client.
func buildStoreResources(op *kunstructured.Unstructured, in *Input) ([]kunstructured.Unstructured, error) {
	resources := make([]kunstructured.Unstructured, 0, 1+len(in.RequiredResources)+len(in.Credentials))

	// The Operation itself.
	resources = append(resources, *op)

	// Required resources.
	resources = append(resources, in.RequiredResources...)

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

// buildOutput assembles an Output from the fake client's captured state.
func buildOutput(c *render.InMemoryClient, recorder *render.EventRecorder) *Output {
	out := &Output{
		APIVersion:       APIVersion,
		Kind:             KindOutput,
		AppliedResources: []kunstructured.Unstructured{},
	}

	// The final Operation state is the last Status().Update call for the
	// Operation GVK.
	opGVK := opsv1alpha1.OperationGroupVersionKind
	for i := len(c.Updated()) - 1; i >= 0; i-- {
		u := c.Updated()[i]
		if u.GroupVersionKind() == opGVK {
			out.Operation = u
			break
		}
	}

	// Collect applied resources (SSA Patch calls). Exclude the Operation
	// itself.
	for _, u := range c.Applied() {
		if u.GroupVersionKind() == opGVK {
			continue
		}
		out.AppliedResources = append(out.AppliedResources, u)
	}

	// Collect events.
	out.Events = append(out.Events, recorder.Events()...)

	return out
}

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
	"time"

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
	cronrec "github.com/crossplane/crossplane/v2/internal/controller/ops/cronoperation"
	oprec "github.com/crossplane/crossplane/v2/internal/controller/ops/operation"
	watchrec "github.com/crossplane/crossplane/v2/internal/controller/ops/watched"
	"github.com/crossplane/crossplane/v2/internal/render"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Render runs one real Operation reconcile loop using the real reconciler
// engine backed by a fake in-memory client.
func Render(ctx context.Context, log logging.Logger, in *renderv1alpha1.OperationInput) (*renderv1alpha1.OperationOutput, error) {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add core/v1 to scheme")
	}
	if err := apis.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add Crossplane APIs to scheme")
	}

	// Convert the Operation from protobuf Struct to unstructured.
	opUnstructured := &kunstructured.Unstructured{}
	if err := xfn.FromStruct(opUnstructured, in.GetOperation()); err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation from protobuf")
	}
	opUnstructured.SetGroupVersionKind(opsv1alpha1.OperationGroupVersionKind)
	opUnstructured.SetResourceVersion("999")

	// Build the in-memory store with all input resources.
	storeResources, err := buildStoreResources(opUnstructured, in)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build store resources")
	}
	fakeClient := render.NewInMemoryClient(s, storeResources...)

	// Connect to function gRPC servers.
	runner, err := render.NewFunctionRunner(in.GetFunctions())
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
				return nil
			},
		)),
		oprec.WithRecorder(recorder),
		oprec.WithLogger(log),
	)

	// Run one reconcile loop.
	req := reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: opUnstructured.GetNamespace(),
		Name:      opUnstructured.GetName(),
	}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		return nil, errors.Wrap(err, "reconcile failed")
	}

	return buildOutput(fakeClient, recorder)
}

// NewFromCronOperation produces the Operation a CronOperation would create.
func NewFromCronOperation(in *renderv1alpha1.CronOperationInput) (*renderv1alpha1.CronOperationOutput, error) {
	co := &opsv1alpha1.CronOperation{}
	if err := xfn.FromStruct(co, in.GetCronOperation()); err != nil {
		return nil, errors.Wrap(err, "cannot convert CronOperation from protobuf")
	}

	scheduled := time.Now()
	if in.GetScheduledUnix() != 0 {
		scheduled = time.Unix(in.GetScheduledUnix(), 0)
	}

	op := cronrec.NewOperation(co, scheduled)
	opStruct, err := xfn.AsStruct(op)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
	}

	return &renderv1alpha1.CronOperationOutput{Operation: opStruct}, nil
}

// NewFromWatchOperation produces the Operation a WatchOperation would create.
func NewFromWatchOperation(in *renderv1alpha1.WatchOperationInput) (*renderv1alpha1.WatchOperationOutput, error) {
	wo := &opsv1alpha1.WatchOperation{}
	if err := xfn.FromStruct(wo, in.GetWatchOperation()); err != nil {
		return nil, errors.Wrap(err, "cannot convert WatchOperation from protobuf")
	}

	watched := &kunstructured.Unstructured{}
	if err := xfn.FromStruct(watched, in.GetWatchedResource()); err != nil {
		return nil, errors.Wrap(err, "cannot convert watched resource from protobuf")
	}

	name := watchrec.OperationName(wo, watched)
	op := watchrec.NewOperation(wo, watched, name)
	opStruct, err := xfn.AsStruct(op)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
	}

	return &renderv1alpha1.WatchOperationOutput{Operation: opStruct}, nil
}

// buildStoreResources assembles all resources to load into the fake client.
func buildStoreResources(op *kunstructured.Unstructured, in *renderv1alpha1.OperationInput) ([]kunstructured.Unstructured, error) {
	resources := make([]kunstructured.Unstructured, 0, 1+len(in.GetRequiredResources())+len(in.GetCredentials()))

	resources = append(resources, *op)

	for _, s := range in.GetRequiredResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert required resource from protobuf")
		}
		resources = append(resources, *u)
	}

	for _, s := range in.GetCredentials() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert credential from protobuf")
		}
		if u.GetKind() == "" {
			u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
		}
		resources = append(resources, *u)
	}

	return resources, nil
}

// buildOutput assembles an OperationOutput from the fake client's captured
// state.
func buildOutput(c *render.InMemoryClient, recorder *render.EventRecorder) (*renderv1alpha1.OperationOutput, error) {
	out := &renderv1alpha1.OperationOutput{}

	opGVK := opsv1alpha1.OperationGroupVersionKind
	for i := len(c.Updated()) - 1; i >= 0; i-- {
		u := c.Updated()[i]
		if u.GroupVersionKind() == opGVK {
			s, err := xfn.AsStruct(&u)
			if err != nil {
				return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
			}
			out.Operation = s
			break
		}
	}

	for _, u := range c.Applied() {
		if u.GroupVersionKind() == opGVK {
			continue
		}
		s, err := xfn.AsStruct(&u)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert applied resource to protobuf")
		}
		out.AppliedResources = append(out.AppliedResources, s)
	}

	out.Events = append(out.Events, recorder.Events()...)

	return out, nil
}

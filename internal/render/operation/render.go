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

// Package operation renders an Operation by running one real reconcile loop
// against a fake in-memory client.
package operation

import (
	"context"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
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

	op := &kunstructured.Unstructured{}
	if err := xfn.FromStruct(op, in.GetOperation()); err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation from protobuf")
	}
	op.SetGroupVersionKind(opsv1alpha1.OperationGroupVersionKind)
	op.SetResourceVersion("999")

	// Build the in-memory store with all input resources.
	store := []kunstructured.Unstructured{*op}
	for _, s := range in.GetRequiredResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert required resource from protobuf")
		}
		store = append(store, *u)
	}
	for _, s := range in.GetCredentials() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return nil, errors.Wrap(err, "cannot convert credential from protobuf")
		}
		if u.GetKind() == "" {
			u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
		}
		store = append(store, *u)
	}
	c := render.NewInMemoryClient(s, store...)

	runner, err := render.NewFunctionRunner(in.GetFunctions())
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to functions")
	}
	defer runner.Close() //nolint:errcheck // Best-effort cleanup.

	oc, err := render.NewInMemoryOpenAPIClient(in.GetRequiredSchemas())
	if err != nil {
		return nil, errors.Wrap(err, "cannot build OpenAPI client from input schemas")
	}
	sf := xfn.NewOpenAPIRequiredSchemasFetcher(oc)
	rsf := render.NewRecordingRequiredSchemasFetcher(sf)
	rrf := render.NewRecordingRequiredResourcesFetcher(xfn.NewExistingRequiredResourcesFetcher(c))

	rec := &render.EventRecorder{}

	r := oprec.NewReconciler(c,
		oprec.WithFunctionRunner(xfn.NewFetchingFunctionRunner(runner, rrf, rsf)),
		oprec.WithCapabilityChecker(xfn.CapabilityCheckerFn(
			func(_ context.Context, _ []string, _ ...string) error {
				return nil
			},
		)),
		oprec.WithRequiredResourcesFetcher(rrf),
		oprec.WithRequiredSchemasFetcher(rsf),
		oprec.WithRecorder(rec),
		oprec.WithLogger(log),
	)

	req := reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: op.GetNamespace(),
		Name:      op.GetName(),
	}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		return nil, errors.Wrap(err, "reconcile failed")
	}

	opGVK := opsv1alpha1.OperationGroupVersionKind
	return buildOutput(c, func(u kunstructured.Unstructured) bool {
		return u.GroupVersionKind() == opGVK &&
			u.GetNamespace() == op.GetNamespace() &&
			u.GetName() == op.GetName()
	}, rec, rrf, rsf)
}

// NewFromCronOperation produces the Operation a CronOperation would create.
func NewFromCronOperation(in *renderv1alpha1.CronOperationInput) (*renderv1alpha1.CronOperationOutput, error) {
	co := &opsv1alpha1.CronOperation{}
	if err := xfn.FromStruct(co, in.GetCronOperation()); err != nil {
		return nil, errors.Wrap(err, "cannot convert CronOperation from protobuf")
	}

	scheduled := time.Now()
	if in.GetScheduledTime() != nil {
		scheduled = in.GetScheduledTime().AsTime()
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

	op := watchrec.NewOperation(wo, watched, watchrec.OperationName(wo, watched))
	opStruct, err := xfn.AsStruct(op)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
	}

	return &renderv1alpha1.WatchOperationOutput{Operation: opStruct}, nil
}

// buildOutput assembles an OperationOutput from the fake client's captured
// state. The isPrimary predicate identifies the primary resource (the
// Operation) so it can be separated from applied resources.
func buildOutput(c *render.InMemoryClient, isPrimary func(kunstructured.Unstructured) bool, rec *render.EventRecorder, rrf *render.RecordingRequiredResourcesFetcher, rsf *render.RecordingRequiredSchemasFetcher) (*renderv1alpha1.OperationOutput, error) {
	out := &renderv1alpha1.OperationOutput{}

	for i := len(c.Updated()) - 1; i >= 0; i-- {
		u := c.Updated()[i]
		if isPrimary(u) {
			s, err := xfn.AsStruct(&u)
			if err != nil {
				return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
			}
			out.Operation = s
			break
		}
	}

	for _, u := range c.Applied() {
		if isPrimary(u) {
			continue
		}
		s, err := xfn.AsStruct(&u)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert applied resource to protobuf")
		}
		out.AppliedResources = append(out.AppliedResources, s)
	}

	out.Events = append(out.Events, rec.Events()...)

	// Collect required resource selectors.
	for _, rs := range rrf.GetResourceSelectors() {
		s, err := messageToStruct(rs)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert required resource selector to struct")
		}
		out.RequiredResources = append(out.RequiredResources, s)
	}

	// Collect required schmea selectors.
	for _, ss := range rsf.GetSchemaSelectors() {
		s, err := messageToStruct(ss)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert required schema selector to struct")
		}
		out.RequiredSchemas = append(out.RequiredSchemas, s)
	}

	return out, nil
}

func messageToStruct(m proto.Message) (*structpb.Struct, error) {
	bs, err := protojson.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal message to json")
	}
	s := &structpb.Struct{}
	if err := s.UnmarshalJSON(bs); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal message from json")
	}

	return s, nil
}

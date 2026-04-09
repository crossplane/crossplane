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

package render

import (
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// buildCompositeRequest builds a RenderRequest for a composite resource from
// the supplied inputs and function addresses.
func buildCompositeRequest(in CompositionInputs) (*renderv1alpha1.RenderRequest, error) {
	xrStruct, err := xfn.AsStruct(in.CompositeResource)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert composite resource to protobuf")
	}

	compStruct, err := asStructFromTyped(in.Composition)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Composition to protobuf")
	}

	fnInputs := make([]*renderv1alpha1.FunctionInput, 0, len(in.FunctionAddrs))
	for name, addr := range in.FunctionAddrs {
		fnInputs = append(fnInputs, &renderv1alpha1.FunctionInput{
			Name:    name,
			Address: addr,
		})
	}

	observedStructs, err := composedToStructs(in.ObservedResources)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert observed resources to protobuf")
	}

	requiredStructs, err := unstructuredToStructs(in.RequiredResources)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert required resources to protobuf")
	}

	credStructs, err := secretsToStructs(in.FunctionCredentials)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert credentials to protobuf")
	}

	schemaStructs, err := schemasToStructs(in.RequiredSchemas)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert required schemas to protobuf")
	}

	return &renderv1alpha1.RenderRequest{
		Meta: &renderv1alpha1.RequestMeta{},
		Input: &renderv1alpha1.RenderRequest_Composite{
			Composite: &renderv1alpha1.CompositeInput{
				CompositeResource: xrStruct,
				Composition:       compStruct,
				Functions:         fnInputs,
				ObservedResources: observedStructs,
				RequiredResources: requiredStructs,
				RequiredSchemas:   schemaStructs,
				Credentials:       credStructs,
			},
		},
	}, nil
}

// parseCompositeResponse converts a CompositeOutput into Outputs for the CLI.
func parseCompositeResponse(out *renderv1alpha1.CompositeOutput) (CompositionOutputs, error) {
	xr := ucomposite.New()
	if s := out.GetCompositeResource(); s != nil {
		if err := xfn.FromStruct(xr, s); err != nil {
			return CompositionOutputs{}, errors.Wrap(err, "cannot convert composite resource from protobuf")
		}
	}

	cds := make([]composed.Unstructured, 0, len(out.GetComposedResources()))
	for _, s := range out.GetComposedResources() {
		cd := composed.New()
		if err := xfn.FromStruct(cd, s); err != nil {
			return CompositionOutputs{}, errors.Wrap(err, "cannot convert composed resource from protobuf")
		}
		cds = append(cds, *cd)
	}

	results := make([]kunstructured.Unstructured, 0, len(out.GetEvents()))
	for _, ev := range out.GetEvents() {
		results = append(results, kunstructured.Unstructured{Object: map[string]any{
			"apiVersion": "render.crossplane.io/v1beta1",
			"kind":       "Result",
			"severity":   ev.GetType(),
			"reason":     ev.GetReason(),
			"message":    ev.GetMessage(),
		}})
	}

	return CompositionOutputs{
		CompositeResource: xr,
		ComposedResources: cds,
		Results:           results,
	}, nil
}

// BuildOperationRequest builds a RenderRequest for an Operation from the
// supplied inputs and function addresses.
func BuildOperationRequest(in OperationInputs) (*renderv1alpha1.RenderRequest, error) {
	opStruct, err := asStructFromTyped(in.Operation)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert Operation to protobuf")
	}

	fnInputs := make([]*renderv1alpha1.FunctionInput, 0, len(in.FunctionAddrs))
	for name, addr := range in.FunctionAddrs {
		fnInputs = append(fnInputs, &renderv1alpha1.FunctionInput{
			Name:    name,
			Address: addr,
		})
	}

	requiredStructs, err := unstructuredToStructs(in.RequiredResources)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert required resources to protobuf")
	}

	schemaStructs, err := schemasToStructs(in.RequiredSchemas)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert required schemas to protobuf")
	}

	credStructs, err := secretsToStructs(in.FunctionCredentials)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert credentials to protobuf")
	}

	return &renderv1alpha1.RenderRequest{
		Meta: &renderv1alpha1.RequestMeta{},
		Input: &renderv1alpha1.RenderRequest_Operation{
			Operation: &renderv1alpha1.OperationInput{
				Operation:         opStruct,
				Functions:         fnInputs,
				RequiredResources: requiredStructs,
				RequiredSchemas:   schemaStructs,
				Credentials:       credStructs,
			},
		},
	}, nil
}

// ParseOperationResponse converts an OperationOutput into unstructured types
// for the CLI.
func ParseOperationResponse(out *renderv1alpha1.OperationOutput) (OperationOutputs, error) {
	var op *opsv1alpha1.Operation
	if s := out.GetOperation(); s != nil {
		if err := xfn.FromStruct(op, s); err != nil {
			return OperationOutputs{}, errors.Wrap(err, "cannot convert Operation from protobuf")
		}
	}

	applied := make([]kunstructured.Unstructured, 0, len(out.GetAppliedResources()))
	for _, s := range out.GetAppliedResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, s); err != nil {
			return OperationOutputs{}, errors.Wrap(err, "cannot convert applied resource from protobuf")
		}
		applied = append(applied, *u)
	}

	results := make([]kunstructured.Unstructured, 0, len(out.GetEvents()))
	for _, ev := range out.GetEvents() {
		results = append(results, kunstructured.Unstructured{Object: map[string]any{
			"apiVersion": "render.crossplane.io/v1beta1",
			"kind":       "Result",
			"severity":   ev.GetType(),
			"reason":     ev.GetReason(),
			"message":    ev.GetMessage(),
		}})
	}

	return OperationOutputs{
		Operation:        op,
		AppliedResources: applied,
		Results:          results,
	}, nil
}

func asStructFromTyped(o runtime.Object) (*structpb.Struct, error) {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert typed object to unstructured")
	}
	u := &kunstructured.Unstructured{Object: data}
	return xfn.AsStruct(u)
}

func composedToStructs(resources []composed.Unstructured) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, 0, len(resources))
	for i := range resources {
		s, err := xfn.AsStruct(&resources[i])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func unstructuredToStructs(resources []kunstructured.Unstructured) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, 0, len(resources))
	for i := range resources {
		s, err := xfn.AsStruct(&resources[i])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func schemasToStructs(schemas []spec3.OpenAPI) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, len(schemas))
	for i, schema := range schemas {
		bs, err := schema.MarshalJSON()
		if err != nil {
			return nil, err
		}

		out[i] = &structpb.Struct{}
		if err := out[i].UnmarshalJSON(bs); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func secretsToStructs(secrets []corev1.Secret) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, 0, len(secrets))
	for i := range secrets {
		s, err := asStructFromTyped(&secrets[i])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

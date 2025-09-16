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

package op

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/cmd/crank/render"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Inputs contains all inputs to the operation render process.
type Inputs struct {
	Operation           *opsv1alpha1.Operation
	Functions           []pkgv1.Function
	FunctionCredentials []corev1.Secret
	RequiredResources   []unstructured.Unstructured
	Context             map[string][]byte
}

// Outputs contains all outputs from the operation render process.
type Outputs struct {
	// The rendered Operation. Unstructured to omit spec field without
	// serializing default values.
	Operation *unstructured.Unstructured
	// The desired resources the operation would mutate
	Resources []unstructured.Unstructured
	// The Function results (not render results)
	Results []unstructured.Unstructured
	// The Crossplane context object
	Context *unstructured.Unstructured
}

// Render the desired resources an Operation would create or mutate, given the supplied inputs.
func Render(ctx context.Context, log logging.Logger, in Inputs) (Outputs, error) { //nolint:gocognit // Operation rendering pipeline is complex but focused.
	runtimes, err := render.NewRuntimeFunctionRunner(ctx, log, in.Functions)
	if err != nil {
		return Outputs{}, errors.Wrap(err, "cannot start function runtimes")
	}

	defer func() { //nolint:contextcheck // See comment on next line.
		// Don't use the main context, since it may be cancelled by the time we
		// get to cleanup (e.g., if render times out).
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := runtimes.Stop(stopCtx); err != nil {
			log.Info("Error stopping function runtimes", "error", err)
		}
	}()

	runner := xfn.NewFetchingFunctionRunner(runtimes, render.NewFilteringFetcher(in.RequiredResources...))

	// Build the function context from supplied context data
	fctx := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	for k, data := range in.Context {
		var jv any
		if err := json.Unmarshal(data, &jv); err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot unmarshal JSON value for context key %q", k)
		}

		v, err := structpb.NewValue(jv)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot store JSON value for context key %q", k)
		}
		fctx.Fields[k] = v
	}

	// Run the operation pipeline - similar to Operation controller logic
	d := &fnv1.State{}
	results := make([]unstructured.Unstructured, 0)
	pipeline := []opsv1alpha1.PipelineStepStatus{}

	for _, fn := range in.Operation.Spec.Pipeline {
		req := &fnv1.RunFunctionRequest{Desired: d, Context: fctx}

		// Handle function input
		if fn.Input != nil {
			input := &structpb.Struct{}
			if err := input.UnmarshalJSON(fn.Input.Raw); err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot unmarshal input for operation pipeline step %q", fn.Step)
			}
			req.Input = input
		}

		// Handle function credentials
		req.Credentials = map[string]*fnv1.Credentials{}
		for _, cs := range fn.Credentials {
			if cs.Source != opsv1alpha1.FunctionCredentialsSourceSecret || cs.SecretRef == nil {
				continue
			}

			s, err := render.GetSecret(cs.SecretRef.Name, cs.SecretRef.Namespace, in.FunctionCredentials)
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot get credentials from secret %q", cs.SecretRef.Name)
			}

			req.Credentials[cs.Name] = &fnv1.Credentials{
				Source: &fnv1.Credentials_CredentialData{
					CredentialData: &fnv1.CredentialData{
						Data: s.Data,
					},
				},
			}
		}

		// Handle bootstrap requirements
		if fn.Requirements != nil {
			req.RequiredResources = map[string]*fnv1.Resources{}
			for _, sel := range fn.Requirements.RequiredResources {
				resources, err := render.NewFilteringFetcher(in.RequiredResources...).Fetch(ctx, xfn.ToProtobufResourceSelector(&sel))
				if err != nil {
					return Outputs{}, errors.Wrapf(err, "cannot fetch bootstrap required resources for requirement %q", sel.RequirementName)
				}
				req.RequiredResources[sel.RequirementName] = resources
			}
		}

		req.Meta = &fnv1.RequestMeta{Tag: xfn.Tag(req)}

		rsp, err := runner.RunFunction(ctx, fn.FunctionRef.Name, req)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot run operation pipeline step %q", fn.Step)
		}

		// Pass desired state to next function
		d = rsp.GetDesired()
		fctx = rsp.GetContext()

		// Handle function output (similar to controller logic)
		if o := rsp.GetOutput(); o != nil {
			j, err := protojson.Marshal(o)
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot marshal pipeline step %q output to JSON", fn.Step)
			}
			pipeline = AddPipelineStepOutput(pipeline, fn.Step, &runtime.RawExtension{Raw: j})
		}

		// Handle fatal results
		for _, rs := range rsp.GetResults() {
			switch rs.GetSeverity() { //nolint:exhaustive // We intentionally have a broad default case.
			case fnv1.Severity_SEVERITY_FATAL:
				return Outputs{}, errors.Errorf("pipeline step %q returned a fatal result: %s", fn.Step, rs.GetMessage())
			default:
				results = append(results, unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "render.crossplane.io/v1beta1",
					"kind":       "Result",
					"step":       fn.Step,
					"severity":   rs.GetSeverity().String(),
					"message":    rs.GetMessage(),
				}})
			}
		}
	}

	// Convert desired resources to unstructured
	resources := make([]unstructured.Unstructured, 0)
	for name, dr := range d.GetResources() {
		u := &unstructured.Unstructured{}
		if err := xfn.FromStruct(u, dr.GetResource()); err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot load desired resource %q from protobuf struct", name)
		}
		resources = append(resources, *u)
	}

	// Build updated Operation with status from the function pipeline
	// Create unstructured Operation with only metadata (no spec)
	op := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": in.Operation.APIVersion,
			"kind":       in.Operation.Kind,
			"metadata":   in.Operation.ObjectMeta.DeepCopy(),
		},
	}

	// Build the Operation status similar to how the controller does it
	// Track applied resource references for all resources that would be applied
	arr := []opsv1alpha1.AppliedResourceRef{}
	for _, resource := range resources {
		ref := opsv1alpha1.AppliedResourceRef{
			APIVersion: resource.GetAPIVersion(),
			Kind:       resource.GetKind(),
			Name:       resource.GetName(),
		}
		if ns := resource.GetNamespace(); ns != "" {
			ref.Namespace = &ns
		}
		arr = append(arr, ref)
	}
	// Set the status in the unstructured Operation
	_ = fieldpath.Pave(op.Object).SetValue("status", map[string]any{
		"appliedResourceRefs": arr,
		"pipeline":            pipeline,
	})

	// Build context output if available and non-empty
	var uctx *unstructured.Unstructured
	if fctx != nil && len(fctx.GetFields()) > 0 {
		// Convert structpb fields to regular Go values
		fields := make(map[string]any)
		for k, v := range fctx.GetFields() {
			fields[k] = v.AsInterface()
		}
		uctx = &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "render.crossplane.io/v1beta1",
			"kind":       "Context",
			"fields":     fields,
		}}
	}

	return Outputs{
		Operation: op,
		Resources: resources,
		Results:   results,
		Context:   uctx,
	}, nil
}

// AddPipelineStepOutput updates the output for a pipeline step in the
// supplied pipeline status slice. If the step already exists, its output is
// updated in place. If it doesn't exist, it's appended to the slice. The input
// slice is assumed to be sorted by step name.
func AddPipelineStepOutput(pipeline []opsv1alpha1.PipelineStepStatus, step string, output *runtime.RawExtension) []opsv1alpha1.PipelineStepStatus {
	for i, ps := range pipeline {
		if ps.Step == step {
			pipeline[i].Output = output
			return pipeline
		}
	}

	return append(pipeline, opsv1alpha1.PipelineStepStatus{
		Step:   step,
		Output: output,
	})
}

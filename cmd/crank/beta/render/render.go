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
	"encoding/json"
	"reflect"
	"sort"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
)

// Wait for the server to be ready before sending RPCs. Notably this gives
// Docker containers time to start before we make a request. See
// https://grpc.io/docs/guides/wait-for-ready/
const waitForReady = `{
	"methodConfig":[{
		"name": [{"service": "apiextensions.fn.proto.v1beta1.FunctionRunnerService"}],
		"waitForReady": true
	}]
}`

// Annotations added to composed resources.
const (
	AnnotationKeyCompositionResourceName = "crossplane.io/composition-resource-name"
	AnnotationKeyCompositeName           = "crossplane.io/composite"
	AnnotationKeyClaimNamespace          = "crossplane.io/claim-namespace"
	AnnotationKeyClaimName               = "crossplane.io/claim-name"
)

// Inputs contains all inputs to the render process.
type Inputs struct {
	CompositeResource *ucomposite.Unstructured
	Composition       *apiextensionsv1.Composition
	Functions         []pkgv1beta1.Function
	ObservedResources []composed.Unstructured
	ExtraResources    []unstructured.Unstructured
	Context           map[string][]byte

	// TODO(negz): Allow supplying observed XR and composed resource connection
	// details. Maybe as Secrets? What if secret stores are in use?
}

// Outputs contains all outputs from the render process.
type Outputs struct {
	CompositeResource *ucomposite.Unstructured
	ComposedResources []composed.Unstructured
	Results           []unstructured.Unstructured
	Context           *unstructured.Unstructured

	// TODO(negz): Allow returning desired XR connection details. Maybe as a
	// Secret? Should we honor writeConnectionSecretToRef? What if secret stores
	// are in use?

	// TODO(negz): Allow returning desired XR readiness? Or perhaps just set the
	// ready status condition on the XR if all supplied observed resources
	// appear ready?
}

// Render the desired XR and composed resources, sorted by resource name, given the supplied inputs.
func Render(ctx context.Context, in Inputs) (Outputs, error) { //nolint:gocyclo // TODO(negz): Should we refactor to break this up a bit?
	// Run our Functions.
	conns := map[string]*grpc.ClientConn{}
	for _, fn := range in.Functions {
		runtime, err := GetRuntime(fn)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot get runtime for Function %q", fn.GetName())
		}
		rctx, err := runtime.Start(ctx)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot start Function %q", fn.GetName())
		}
		defer rctx.Stop(ctx) //nolint:errcheck // Not sure what to do with this error. Log it to stderr?

		conn, err := grpc.DialContext(ctx, rctx.Target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(waitForReady))
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot dial Function %q at address %q", fn.GetName(), rctx.Target)
		}
		defer conn.Close() //nolint:errcheck // This only returns an error if the connection is already closed or closing.
		conns[fn.GetName()] = conn
	}

	observed := composite.ComposedResourceStates{}
	for i, cd := range in.ObservedResources {
		name := cd.GetAnnotations()[AnnotationKeyCompositionResourceName]
		observed[composite.ResourceName(name)] = composite.ComposedResourceState{
			Resource:          &in.ObservedResources[i],
			ConnectionDetails: nil, // We don't support passing in observed connection details.
			Ready:             false,
		}
	}

	// TODO(negz): Support passing in optional observed connection details for
	// both the XR and composed resources.
	o, err := composite.AsState(in.CompositeResource, nil, observed)
	if err != nil {
		return Outputs{}, errors.Wrap(err, "cannot build observed composite and composed resources for RunFunctionRequest")
	}

	// The Function pipeline starts with empty desired state.
	d := &fnv1beta1.State{}

	results := make([]unstructured.Unstructured, 0)

	// The Function context starts empty.
	fctx := &structpb.Struct{Fields: map[string]*structpb.Value{}}

	// Load user-supplied context.
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

	// Run any Composition Functions in the pipeline. Each Function may mutate
	// the desired state returned by the last, and each Function may produce
	// results.
	for _, fn := range in.Composition.Spec.Pipeline {
		conn, ok := conns[fn.FunctionRef.Name]
		if !ok {
			return Outputs{}, errors.Errorf("unknown Function %q, referenced by pipeline step %q - does it exist in your Functions file?", fn.FunctionRef.Name, fn.Step)
		}

		fClient := fnv1beta1.NewFunctionRunnerServiceClient(conn)

		// The request to send to the function, will be updated at each iteration if needed.
		req := &fnv1beta1.RunFunctionRequest{Observed: o, Desired: d, Context: fctx}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot unmarshal input for Composition pipeline step %q", fn.Step)
			}
			req.Input = in
		}

		// Used to store the requirements returned at the previous iteration.
		var requirements *fnv1beta1.Requirements
		// Used to store the response of the function at the previous iteration.
		var rsp *fnv1beta1.RunFunctionResponse

		for i := int64(0); i <= composite.MaxRequirementsIterations; i++ {
			if i == composite.MaxRequirementsIterations {
				// The requirements didn't stabilize after the maximum number of iterations.
				return Outputs{}, errors.Errorf("requirements didn't stabilize after the maximum number of iterations (%d)", composite.MaxRequirementsIterations)
			}

			rsp, err = fClient.RunFunction(ctx, req)
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot run pipeline step %q", fn.Step)
			}

			newRequirements := rsp.GetRequirements()
			if reflect.DeepEqual(newRequirements, requirements) {
				// The requirements are stable, the function is done.
				break
			}

			// Store the requirements for the next iteration.
			requirements = newRequirements

			// Cleanup the extra resources from the previous iteration to store the new ones
			req.ExtraResources = make(map[string]*fnv1beta1.Resources)

			// Fetch the requested resources and add them to the desired state.
			for name, selector := range newRequirements.GetExtraResources() {
				newExtraResources, err := filterExtraResources(in.ExtraResources, selector)
				if err != nil {
					return Outputs{}, errors.Wrapf(err, "cannot filter extra resources for pipeline step %q", fn.Step)
				}

				// Resources would be nil in case of not found resources.
				req.ExtraResources[name] = newExtraResources
			}

			// Pass down the updated context across iterations.
			req.Context = rsp.GetContext()
		}

		// Pass the desired state returned by this Function to the next one.
		d = rsp.GetDesired()

		// Pass the Function context returned by this Function to the next one.
		// We intentionally discard/ignore this after the last Function runs.
		fctx = rsp.GetContext()

		// Results of fatal severity stop the Composition process.
		for _, rs := range rsp.GetResults() {
			switch rs.GetSeverity() { //nolint:exhaustive // We intentionally have a broad default case.
			case fnv1beta1.Severity_SEVERITY_FATAL:
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

	desired := make([]composed.Unstructured, 0, len(d.GetResources()))
	for name, dr := range d.GetResources() {
		cd := composed.New()
		if err := composite.FromStruct(cd, dr.GetResource()); err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot unmarshal desired composed resource %q", name)
		}

		// If this desired resource state pertains to an existing composed
		// resource we want to maintain its name and namespace.
		or, ok := observed[composite.ResourceName(name)]
		if ok {
			cd.SetNamespace(or.Resource.GetNamespace())
			cd.SetName(or.Resource.GetName())
		}

		// Set standard composed resource metadata that is derived from the XR.
		if err := SetComposedResourceMetadata(cd, in.CompositeResource, name); err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot render composed resource %q metadata", name)
		}

		desired = append(desired, *cd)
	}

	// Sort the resource names to ensure a deterministic ordering of returned composed resources.
	sort.Slice(desired, func(i, j int) bool {
		return desired[i].GetAnnotations()[AnnotationKeyCompositionResourceName] < desired[j].GetAnnotations()[AnnotationKeyCompositionResourceName]
	})

	xr := ucomposite.New()
	if err := composite.FromStruct(xr, d.GetComposite().GetResource()); err != nil {
		return Outputs{}, errors.Wrap(err, "cannot render desired composite resource")
	}

	// The Function pipeline can only return the desired status of the XR, so we
	// inject these back in to help identify which resource it is.
	xr.SetAPIVersion(in.CompositeResource.GetAPIVersion())
	xr.SetKind(in.CompositeResource.GetKind())
	xr.SetName(in.CompositeResource.GetName())

	out := Outputs{CompositeResource: xr, ComposedResources: desired, Results: results}
	if fctx != nil {
		out.Context = &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "render.crossplane.io/v1beta1",
			"kind":       "Context",
			"fields":     fctx.GetFields(),
		}}
	}
	return out, nil
}

// SetComposedResourceMetadata sets standard, required composed resource
// metadata. It's a simplified version of the same function used by Crossplane.
// Notably it doesn't handle 'nested' XRs - it assumes the supplied XR should be
// treated as the top-level XR for setting the crossplane.io/composite,
// crossplane.io/claim-namespace, and crossplane.io/claim-name annotations.
//
// https://github.com/crossplane/crossplane/blob/0965f0/internal/controller/apiextensions/composite/composition_render.go#L117
func SetComposedResourceMetadata(cd resource.Object, xr resource.Composite, name string) error {
	cd.SetGenerateName(xr.GetName() + "-")
	meta.AddAnnotations(cd, map[string]string{AnnotationKeyCompositionResourceName: name})
	meta.AddLabels(cd, map[string]string{AnnotationKeyCompositeName: xr.GetName()})
	if ref := xr.GetClaimReference(); ref != nil {
		meta.AddLabels(cd, map[string]string{
			AnnotationKeyClaimNamespace: ref.Namespace,
			AnnotationKeyClaimName:      ref.Name,
		})
	}

	or := meta.AsController(meta.TypedReferenceTo(xr, xr.GetObjectKind().GroupVersionKind()))
	return errors.Wrapf(meta.AddControllerReference(cd, or), "cannot set composite resource %q as controller ref of composed resource", xr.GetName())
}

func filterExtraResources(ers []unstructured.Unstructured, selector *fnv1beta1.ResourceSelector) (*fnv1beta1.Resources, error) { //nolint:gocyclo // There is not much to simplify here.
	if len(ers) == 0 || selector == nil {
		return nil, nil
	}
	out := &fnv1beta1.Resources{}
	for _, er := range ers {
		er := er
		if selector.GetApiVersion() != er.GetAPIVersion() {
			continue
		}
		if selector.GetKind() != er.GetKind() {
			continue
		}
		if selector.GetMatchName() == er.GetName() {
			o, err := composite.AsStruct(&er)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot marshal extra resource %q", er.GetName())
			}
			out.Items = []*fnv1beta1.Resource{{Resource: o}}
			return out, nil
		}
		if selector.GetMatchLabels() != nil {
			if labels.SelectorFromSet(selector.GetMatchLabels().GetLabels()).Matches(labels.Set(er.GetLabels())) {
				o, err := composite.AsStruct(&er)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot marshal extra resource %q", er.GetName())
				}
				out.Items = append(out.GetItems(), &fnv1beta1.Resource{Resource: o})
			}
		}
	}

	return out, nil
}

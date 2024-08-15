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
	"fmt"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/internal/xfn"
)

// Wait for the server to be ready before sending RPCs. Notably this gives
// Docker containers time to start before we make a request. See
// https://grpc.io/docs/guides/wait-for-ready/
const waitForReady = `{
	"methodConfig":[{
		"name": [{}],
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
	Functions         []pkgv1.Function
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
}

// A RuntimeFunctionRunner is a composite.FunctionRunner that runs functions
// locally, using the runtime configured in their annotations (e.g. Docker).
type RuntimeFunctionRunner struct {
	contexts map[string]RuntimeContext
	conns    map[string]*grpc.ClientConn
	mx       sync.Mutex
}

// NewRuntimeFunctionRunner returns a FunctionRunner that runs functions
// locally, using the runtime configured in their annotations (e.g. Docker). It
// starts all the functions and creates gRPC connections when called.
func NewRuntimeFunctionRunner(ctx context.Context, log logging.Logger, fns []pkgv1.Function) (*RuntimeFunctionRunner, error) {
	contexts := map[string]RuntimeContext{}
	conns := map[string]*grpc.ClientConn{}

	for _, fn := range fns {
		runtime, err := GetRuntime(fn, log)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get runtime for Function %q", fn.GetName())
		}
		rctx, err := runtime.Start(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot start Function %q", fn.GetName())
		}
		contexts[fn.GetName()] = rctx

		conn, err := grpc.DialContext(ctx, rctx.Target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(waitForReady))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot dial Function %q at address %q", fn.GetName(), rctx.Target)
		}
		conns[fn.GetName()] = conn
	}

	return &RuntimeFunctionRunner{conns: conns}, nil
}

// RunFunction runs the named function.
func (r *RuntimeFunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	r.mx.Lock()
	defer r.mx.Unlock()

	conn, ok := r.conns[name]
	if !ok {
		return nil, errors.Errorf("unknown Function %q - does it exist in your Functions file?", name)
	}

	return xfn.NewBetaFallBackFunctionRunnerServiceClient(conn).RunFunction(ctx, req)
}

// Stop all of the runner's runtimes, and close its gRPC connections.
func (r *RuntimeFunctionRunner) Stop(ctx context.Context) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	for name, conn := range r.conns {
		_ = conn.Close()
		delete(r.conns, name)
	}
	for name, rctx := range r.contexts {
		if err := rctx.Stop(ctx); err != nil {
			return errors.Wrapf(err, "cannot stop function %q runtime (target %q)", name, rctx.Target)
		}
		delete(r.contexts, name)
	}

	return nil
}

// Render the desired XR and composed resources, sorted by resource name, given the supplied inputs.
func Render(ctx context.Context, log logging.Logger, in Inputs) (Outputs, error) { //nolint:gocognit // TODO(negz): Should we refactor to break this up a bit?
	runtimes, err := NewRuntimeFunctionRunner(ctx, log, in.Functions)
	if err != nil {
		return Outputs{}, errors.Wrap(err, "cannot start function runtimes")
	}

	defer func() {
		if err := runtimes.Stop(ctx); err != nil {
			log.Info("Error stopping function runtimes", "error", err)
		}
	}()

	runner := composite.NewFetchingFunctionRunner(runtimes, &FilteringFetcher{extra: in.ExtraResources})

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
	d := &fnv1.State{}

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
		// The request to send to the function, will be updated at each iteration if needed.
		req := &fnv1.RunFunctionRequest{Observed: o, Desired: d, Context: fctx}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot unmarshal input for Composition pipeline step %q", fn.Step)
			}
			req.Input = in
		}

		rsp, err := runner.RunFunction(ctx, fn.FunctionRef.Name, req)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "cannot run pipeline step %q", fn.Step)
		}

		// Pass the desired state returned by this Function to the next one.
		d = rsp.GetDesired()

		// Pass the Function context returned by this Function to the next one.
		// We intentionally discard/ignore this after the last Function runs.
		fctx = rsp.GetContext()

		// Results of fatal severity stop the Composition process.
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

	desired := make([]composed.Unstructured, 0, len(d.GetResources()))
	var unready []string
	for name, dr := range d.GetResources() {
		if dr.GetReady() != fnv1.Ready_READY_TRUE {
			unready = append(unready, name)
		}

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
	xrCond := xpv1.Available()
	if len(unready) > 0 {
		xrCond = xpv1.Creating().WithMessage(fmt.Sprintf("Unready resources: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, unready)))
	}
	// lastTransitionTime would just be noise, but we can't drop it as it's a
	// required field and null is not allowed, so we set a random time.
	xrCond.LastTransitionTime = metav1.NewTime(time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC))
	xr.SetConditions(xrCond)

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

// FilteringFetcher is a composite.ExtraResourcesFetcher that "fetches" any
// supplied resource that matches a resource selector.
type FilteringFetcher struct {
	extra []unstructured.Unstructured
}

// Fetch returns all of the underlying extra resources that match the supplied
// resource selector.
func (f *FilteringFetcher) Fetch(_ context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	if len(f.extra) == 0 || rs == nil {
		return nil, nil
	}
	out := &fnv1.Resources{}
	for _, er := range f.extra {
		if rs.GetApiVersion() != er.GetAPIVersion() {
			continue
		}
		if rs.GetKind() != er.GetKind() {
			continue
		}
		if rs.GetMatchName() == er.GetName() {
			o, err := composite.AsStruct(&er)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot marshal extra resource %q", er.GetName())
			}
			out.Items = []*fnv1.Resource{{Resource: o}}
			return out, nil
		}
		if rs.GetMatchLabels() != nil {
			if labels.SelectorFromSet(rs.GetMatchLabels().GetLabels()).Matches(labels.Set(er.GetLabels())) {
				o, err := composite.AsStruct(&er)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot marshal extra resource %q", er.GetName())
				}
				out.Items = append(out.GetItems(), &fnv1.Resource{Resource: o})
			}
		}
	}

	return out, nil
}

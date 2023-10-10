package main

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
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

// RenderInputs contains all inputs to the render process.
type RenderInputs struct {
	CompositeResource *composite.Unstructured
	Composition       *apiextensionsv1.Composition
	Functions         []pkgv1beta1.Function
	ObservedResources []composed.Unstructured

	// TODO(negz): Allow supplying observed XR and composed resource connection
	// details. Maybe as Secrets? What if secret stores are in use?
}

// RenderOutputs contains all outputs from the render process.
type RenderOutputs struct {
	CompositeResource *composite.Unstructured
	ComposedResources []composed.Unstructured
	Results           []unstructured.Unstructured

	// TODO(negz): Allow returning desired XR connection details. Maybe as a
	// Secret? Should we honor writeConnectionSecretToRef? What if secret stores
	// are in use?

	// TODO(negz): Allow returning desired XR readiness? Or perhaps just set the
	// ready status condition on the XR if all supplied observed resources
	// appear ready?
}

// Render the desired XR and composed resources given the supplied inputs.
func Render(ctx context.Context, in RenderInputs) (RenderOutputs, error) { //nolint:gocyclo // TODO(negz): Should we refactor to break this up a bit?

	// Run our Functions.
	conns := map[string]*grpc.ClientConn{}
	for _, fn := range in.Functions {
		runtime, err := GetRuntime(fn)
		if err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot get runtime for Function %q", fn.GetName())
		}
		rctx, err := runtime.Start(ctx)
		if err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot start Function %q", fn.GetName())
		}
		defer rctx.Stop(ctx) //nolint:errcheck // Not sure what to do with this error. Log it to stderr?

		conn, err := grpc.DialContext(ctx, rctx.Target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(waitForReady))
		if err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot dial Function %q at address %q", fn.GetName(), rctx.Target)
		}
		defer conn.Close() //nolint:errcheck // This only returns an error if the connection is already closed or closing.
		conns[fn.GetName()] = conn
	}

	observed := map[string]composed.Unstructured{}
	for _, cd := range in.ObservedResources {
		name := cd.GetAnnotations()[AnnotationKeyCompositionResourceName]
		observed[name] = cd
	}

	// TODO(negz): Support passing in optional observed connection details for
	// both the XR and composed resources.
	o, err := AsState(in.CompositeResource, observed)
	if err != nil {
		return RenderOutputs{}, errors.Wrap(err, "cannot build observed composite and composed resources for RunFunctionRequest")
	}

	// The Function pipeline starts with empty desired state.
	d := &fnv1beta1.State{}

	results := make([]unstructured.Unstructured, 0)

	// Run any Composition Functions in the pipeline. Each Function may mutate
	// the desired state returned by the last, and each Function may produce
	// results.
	for _, fn := range in.Composition.Spec.Pipeline {
		req := &fnv1beta1.RunFunctionRequest{Observed: o, Desired: d}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				return RenderOutputs{}, errors.Wrapf(err, "cannot unmarshal input for Composition pipeline step %q", fn.Step)
			}
			req.Input = in
		}

		conn, ok := conns[fn.FunctionRef.Name]
		if !ok {
			return RenderOutputs{}, errors.Errorf("unknown Function %q, referenced by pipeline step %q - does it exist in your Functions file?", fn.FunctionRef.Name, fn.Step)
		}

		rsp, err := fnv1beta1.NewFunctionRunnerServiceClient(conn).RunFunction(ctx, req)
		if err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot run pipeline step %q", fn.Step)
		}

		d = rsp.GetDesired()

		// Results of fatal severity stop the Composition process.
		for _, rs := range rsp.Results {
			switch rs.Severity { //nolint:exhaustive // We intentionally have a broad default case.
			case fnv1beta1.Severity_SEVERITY_FATAL:
				return RenderOutputs{}, errors.Errorf("pipeline step %q returned a fatal result: %s", fn.Step, rs.Message)
			default:
				results = append(results, unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "xrender.crossplane.io/v1beta1",
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
		if err := FromStruct(cd, dr.GetResource()); err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot unmarshal desired composed resource %q", name)
		}

		// If this desired resource state pertains to an existing composed
		// resource we want to maintain its name and namespace.
		or, ok := observed[name]
		if ok {
			cd.SetNamespace(or.GetNamespace())
			cd.SetName(or.GetName())
		}

		// Set standard composed resource metadata that is derived from the XR.
		if err := RenderComposedResourceMetadata(cd, in.CompositeResource, name); err != nil {
			return RenderOutputs{}, errors.Wrapf(err, "cannot render composed resource %q metadata", name)
		}

		desired = append(desired, *cd)
	}

	xr := composite.New()
	if err := FromStruct(xr, d.GetComposite().GetResource()); err != nil {
		return RenderOutputs{}, errors.Wrap(err, "cannot render desired composite resource")
	}

	// The Function pipeline can only return the desired status of the XR, so we
	// inject these back in to help identify which resource it is.
	xr.SetAPIVersion(in.CompositeResource.GetAPIVersion())
	xr.SetKind(in.CompositeResource.GetKind())
	xr.SetName(in.CompositeResource.GetName())

	return RenderOutputs{CompositeResource: xr, ComposedResources: desired, Results: results}, nil
}

// RenderComposedResourceMetadata sets standard, required composed resource
// metadata. It's a simplified version of the same function used by Crossplane.
// Notably it doesn't handle 'nested' XRs - it assumes the supplied XR should be
// treated as the top-level XR for setting the crossplane.io/composite,
// crossplane.io/claim-namespace, and crossplane.io/claim-name annotations.
//
// https://github.com/crossplane/crossplane/blob/0965f0/internal/controller/apiextensions/composite/composition_render.go#L117
func RenderComposedResourceMetadata(cd resource.Object, xr resource.Composite, name string) error {
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

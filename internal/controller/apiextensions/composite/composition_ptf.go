/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package composite

import (
	"context"
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	iov1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errFetchXRConnectionDetails = "cannot fetch composite resource connection details"
	errGetExistingCDs           = "cannot get existing composed resources"
	errImgPullCfg               = "cannot get xfn image pull config"
	errBuildFunctionIOObserved  = "cannot build FunctionIO observed state"
	errBuildFunctionIODesired   = "cannot build initial FunctionIO desired state"
	errMarshalXR                = "cannot marshal composite resource"
	errMarshalCD                = "cannot marshal composed resource"
	errNewKeychain              = "cannot create a new keychain"
	errPatchAndTransform        = "cannot patch and transform"
	errRunFunctionPipeline      = "cannot run Composition Function pipeline"
	errDeleteUndesiredCDs       = "cannot delete undesired composed resources"
	errApplyXR                  = "cannot apply composite resource"
	errObserveCDs               = "cannot observe composed resources"
	errAnonymousCD              = "encountered composed resource without required \"" + AnnotationKeyCompositionResourceName + "\" annotation"
	errUnmarshalDesiredXR       = "cannot unmarshal desired composite resource from FunctionIO"
	errUnmarshalDesiredCD       = "cannot unmarshal desired composed resource from FunctionIO"
	errMarshalFnIO              = "cannot marshal input FunctionIO"
	errDialRunner               = "cannot dial container runner"
	errRunFnContainer           = "cannot run container"
	errCloseRunner              = "cannot close connection to container runner"
	errUnmarshalFnIO            = "cannot unmarshal output FunctionIO"
	errFatalResult              = "fatal function pipeline result"

	errFmtApplyCD                  = "cannot apply composed resource %q"
	errFmtFetchCDConnectionDetails = "cannot fetch connection details for composed resource %q (a %s named %s)"
	errFmtRenderXR                 = "cannot render composite resource from composed resource %q (a %s named %s)"
	errFmtRunFn                    = "cannot run function %q"

	errFmtUnsupportedFnType        = "unsupported function type %q"
	errFmtParseDesiredCD           = "cannot parse desired composed resource %q from FunctionIO"
	errFmtDeleteCD                 = "cannot delete composed resource %q (a %s named %s)"
	errFmtReadiness                = "cannot determine whether composed resource %q (a %s named %s) is ready"
	errFmtExtractConnectionDetails = "cannot extract connection details from composed resource %q (a %s named %s)"
)

// DefaultTarget is the default function runner target endpoint.
const DefaultTarget = "unix-abstract:crossplane/fn/default.sock"

// A PTFComposer (i.e. Patch, Transform, and Function Composer) supports
// composing resources using both Patch and Transform (P&T) logic and a pipeline
// of Composition Functions. Callers may mix P&T with Composition Functions or
// use only one or the other. It does not support anonymous, unnamed resource
// templates and will panic if it encounters one.
type PTFComposer struct {
	client resource.ClientApplicator

	composite   ptfComposite
	composition ptfComposition
}

type ptfComposite struct {
	managed.ConnectionDetailsFetcher
	ComposedResourceGetter
	ComposedResourceDeleter
	ComposedResourceObserver
}

type ptfComposition struct {
	PatchAndTransformer
	FunctionPipelineRunner
}

// A ComposedResourceGetter gets composed resource state.
type ComposedResourceGetter interface {
	GetComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error)
}

// A ComposedResourceGetterFn gets composed resource state.
type ComposedResourceGetterFn func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error)

// GetComposedResources gets composed resource state.
func (fn ComposedResourceGetterFn) GetComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
	return fn(ctx, xr)
}

// A ComposedResourceDeleter deletes composed resources (and their state).
type ComposedResourceDeleter interface {
	DeleteComposedResources(ctx context.Context, s *PTFCompositionState) error
}

// A ComposedResourceDeleterFn deletes composed resources (and their state).
type ComposedResourceDeleterFn func(ctx context.Context, s *PTFCompositionState) error

// DeleteComposedResources deletes composed resources (and their state).
func (fn ComposedResourceDeleterFn) DeleteComposedResources(ctx context.Context, s *PTFCompositionState) error {
	return fn(ctx, s)
}

// A ComposedResourceObserver derives additional state by observing composed
// resources.
type ComposedResourceObserver interface {
	ObserveComposedResources(ctx context.Context, s *PTFCompositionState) error
}

// An ComposedResourceObserverFn derives additional state by observing composed
// resources.
type ComposedResourceObserverFn func(ctx context.Context, s *PTFCompositionState) error

// ObserveComposedResources derives additional state by observing composed
// resources.
func (fn ComposedResourceObserverFn) ObserveComposedResources(ctx context.Context, s *PTFCompositionState) error {
	return fn(ctx, s)
}

// A PatchAndTransformer runs P&T Composition.
type PatchAndTransformer interface {
	PatchAndTransform(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error
}

// A PatchAndTransformerFn runs P&T Composition.
type PatchAndTransformerFn func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error

// PatchAndTransform runs P&T Composition.
func (fn PatchAndTransformerFn) PatchAndTransform(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
	return fn(ctx, req, s)
}

// A FunctionPipelineRunner runs a pipeline of Composition Functions.
type FunctionPipelineRunner interface {
	RunFunctionPipeline(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error
}

// A FunctionPipelineRunnerFn runs a pipeline of Composition Functions.
type FunctionPipelineRunnerFn func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error

// RunFunctionPipeline runs a pipeline of Composition Functions.
func (fn FunctionPipelineRunnerFn) RunFunctionPipeline(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
	return fn(ctx, req, s, o, d)
}

// A PTFComposerOption is used to configure a PTFComposer.
type PTFComposerOption func(*PTFComposer)

// WithCompositeConnectionDetailsFetcher configures how the PTFComposer should
// get the composite resource's connection details.
func WithCompositeConnectionDetailsFetcher(f managed.ConnectionDetailsFetcher) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composite.ConnectionDetailsFetcher = f
	}
}

// WithComposedResourceGetter configures how the PTFComposer should get existing
// composed resources.
func WithComposedResourceGetter(g ComposedResourceGetter) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composite.ComposedResourceGetter = g
	}
}

// WithComposedResourceDeleter configures how the PTFComposer should delete
// undesired composed resources.
func WithComposedResourceDeleter(d ComposedResourceDeleter) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composite.ComposedResourceDeleter = d
	}
}

// WithComposedResourceObserver configures how the PTFComposer should observe
// composed resources after applying them.
func WithComposedResourceObserver(o ComposedResourceObserver) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composite.ComposedResourceObserver = o
	}
}

// WithPatchAndTransformer configures how the PTFComposer should run Patch &
// Transform (P&T) Composition.
func WithPatchAndTransformer(pt PatchAndTransformer) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composition.PatchAndTransformer = pt
	}
}

// WithFunctionPipelineRunner configures how the PTFComposer should run a
// pipeline of Composition Functions.
func WithFunctionPipelineRunner(r FunctionPipelineRunner) PTFComposerOption {
	return func(p *PTFComposer) {
		p.composition.FunctionPipelineRunner = r
	}
}

// NewPTFComposer returns a new Composer that supports composing resources using
// both Patch and Transform (P&T) logic and a pipeline of Composition Functions.
func NewPTFComposer(kube client.Client, o ...PTFComposerOption) *PTFComposer {
	// TODO(negz): Can we avoid double-wrapping if the supplied client is
	// already wrapped? Or just do away with unstructured.NewClient completely?
	kube = unstructured.NewClient(kube)

	f := NewSecretConnectionDetailsFetcher(kube)

	xfnRunner := &DefaultCompositeFunctionRunner{}

	c := &PTFComposer{
		client: resource.ClientApplicator{Client: kube, Applicator: resource.NewAPIPatchingApplicator(kube)},

		composite: ptfComposite{
			ConnectionDetailsFetcher: f,
			ComposedResourceGetter:   NewExistingComposedResourceGetter(kube, f),
			ComposedResourceDeleter:  NewUndesiredComposedResourceDeleter(kube),
			ComposedResourceObserver: ComposedResourceObserverChain{
				NewConnectionDetailsObserver(ConnectionDetailsExtractorFn(ExtractConnectionDetails)),
				NewReadinessObserver(ReadinessCheckerFn(IsReady)),
			},
		},
		composition: ptfComposition{
			PatchAndTransformer:    NewXRCDPatchAndTransformer(RendererFn(RenderComposite), NewAPIDryRunRenderer(kube)),
			FunctionPipelineRunner: NewFunctionPipeline(ContainerFunctionRunnerFn(xfnRunner.RunFunction)),
		},
	}

	for _, fn := range o {
		fn(c)
	}

	return c
}

// PTFCompositionState is used throughout the PTFComposer to track its state.
type PTFCompositionState struct {
	Composite         resource.Composite
	ConnectionDetails managed.ConnectionDetails
	ComposedResources ComposedResourceStates
	Events            []event.Event
}

// Compose resources using both either the Patch & Transform style resources
// array, the functions array, or both.
func (c *PTFComposer) Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) { //nolint:gocyclo // We probably don't want any further abstraction for the sake of reduced complexity.
	xc, err := c.composite.FetchConnection(ctx, xr)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errFetchXRConnectionDetails)
	}

	cds, err := c.composite.GetComposedResources(ctx, xr)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errGetExistingCDs)
	}

	state := &PTFCompositionState{
		Composite:         xr,
		ConnectionDetails: xc,
		ComposedResources: cds,
		Events:            make([]event.Event, 0),
	}

	// Build observed state to be passed to our Composition Function pipeline.
	// Doing this before we patch and transform ensures we report the state we
	// actually observed before we made any mutations.
	o, err := FunctionIOObserved(state)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errBuildFunctionIOObserved)
	}

	// Run P&T logic, updating the composition state accordingly.
	if err := c.composition.PatchAndTransform(ctx, req, state); err != nil {
		return CompositionResult{}, errors.Wrap(err, errPatchAndTransform)
	}

	// Build the initial desired state to be passed to our Composition Function
	// pipeline. It's expected that each function in the pipeline will mutate
	// this state. It includes any desired state accumulated by the P&T logic.
	d, err := FunctionIODesired(state)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errBuildFunctionIODesired)
	}

	// Run Composition Functions, updating the composition state accordingly.
	// Note that this will replace state.Composite with a new object that was
	// unmarshalled from the function pipeline's desired state.
	if err := c.composition.RunFunctionPipeline(ctx, req, state, o, d); err != nil {
		return CompositionResult{}, errors.Wrap(err, errRunFunctionPipeline)
	}

	// Garbage collect any resources that aren't part of our final desired
	// state. We must do this before we update the XR's resource references to
	// ensure that we don't forget and leak them if a delete fails.
	if err := c.composite.DeleteComposedResources(ctx, state); err != nil {
		return CompositionResult{}, errors.Wrap(err, errDeleteUndesiredCDs)
	}

	// Record references to all desired composed resources.
	UpdateResourceRefs(state)

	// The supplied options ensure we merge rather than replace arrays and
	// objects for which a merge configuration has been specified.
	//
	// Note that at this point state.Composite should be a new object - not the
	// xr that was passed to this Compose method. If this call to Apply changes
	// the XR in the API server (i.e. if it's not a no-op) the xr object that
	// was passed to this method will have a stale meta.resourceVersion. This
	// Subsequent attempts to update that object will therefore fail. This
	// should be okay; the caller should keep trying until this is a no-op.
	ao := mergeOptions(filterPatches(allPatches(state.ComposedResources), patchTypesToXR()...))
	if err := c.client.Apply(ctx, state.Composite, ao...); err != nil {
		return CompositionResult{}, errors.Wrap(err, errApplyXR)
	}

	// We apply all of our composed resources before we observe them and update
	// in the loop below. This ensures that issues observing and processing one
	// composed resource won't block the application of another.
	for _, cd := range state.ComposedResources {
		// Don't try to apply this resource if we didn't render it successfully
		// during Patch & Transform Composition. It's possible that cd.Resource
		// is in a partially rendered state. This would be particularly bad if
		// in that state it could be created successfully, but could not later
		// be updated to its fully rendered desired state. Note that this
		// doesn't mean this resource won't exist; it might have been created
		// previously.
		if cd.TemplateRenderErr != nil {
			continue
		}

		ao := []resource.ApplyOption{resource.MustBeControllableBy(state.Composite.GetUID())}
		if cd.Template != nil {
			ao = append(ao, mergeOptions(filterPatches(cd.Template.Patches, patchTypesFromXR()...))...)
		}
		if err := c.client.Apply(ctx, cd.Resource, ao...); err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtApplyCD, cd.ResourceName)
		}
	}

	// Observe all existing composed resources. This derives the XR's connection
	// details from those of the composed resources. It also runs any readiness
	// checks found in either P&T templates or the FunctionIO desired state.
	if err := c.composite.ObserveComposedResources(ctx, state); err != nil {
		return CompositionResult{}, errors.Wrap(err, "cannot observe composed resources")
	}

	out := make([]ComposedResource, 0, len(state.ComposedResources))
	for _, cd := range state.ComposedResources {
		out = append(out, cd.ComposedResource)
	}

	return CompositionResult{ConnectionDetails: state.ConnectionDetails, Composed: out, Events: state.Events}, nil
}

func allPatches(cds ComposedResourceStates) []v1.Patch {
	out := make([]v1.Patch, 0, len(cds))
	for _, cd := range cds {
		if cd.Template == nil {
			continue
		}
		out = append(out, cd.Template.Patches...)
	}
	return out
}

// An ExistingComposedResourceGetter uses an XR's resource references to load
// any existing composed resources from the API server. It also loads their
// connection details.
type ExistingComposedResourceGetter struct {
	resource client.Reader
	details  managed.ConnectionDetailsFetcher
}

// NewExistingComposedResourceGetter returns a ComposedResourceGetter that
// fetches an XR's existing composed resources.
func NewExistingComposedResourceGetter(c client.Reader, f managed.ConnectionDetailsFetcher) *ExistingComposedResourceGetter {
	return &ExistingComposedResourceGetter{resource: c, details: f}
}

// GetComposedResources begins building composed resource state by
// fetching any existing composed resources referenced by the supplied composite
// resource, as well as their connection details.
func (g *ExistingComposedResourceGetter) GetComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
	cds := ComposedResourceStates{}

	for _, ref := range xr.GetResourceReferences() {
		// The PTComposer writes references to resources that it didn't actually
		// render or create. It has to create these placeholder refs because it
		// supports anonymous (unnamed) resource templates; it needs to be able
		// associate entries a Composition's spec.resources array with entries
		// in an XR's spec.resourceRefs array by their index. These references
		// won't have a name - we won't be able to get them because they don't
		// reference a resource that actually exists.
		if ref.Name == "" {
			continue
		}

		r := composed.New(composed.FromReference(ref))
		nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		err := g.resource.Get(ctx, nn, r)
		if kerrors.IsNotFound(err) {
			// We believe we created this resource, but it doesn't exist.
			continue
		}
		if err != nil {
			return nil, errors.Wrap(err, errGetComposed)
		}

		if c := metav1.GetControllerOf(r); c != nil && c.UID != xr.GetUID() {
			// If we don't control this resource we just pretend it doesn't
			// exist. We might try to render and re-create it later, but that
			// should fail because we check the controller ref there too.
			continue
		}

		name := GetCompositionResourceName(r)
		if name == "" {
			return nil, errors.New(errAnonymousCD)
		}

		conn, err := g.details.FetchConnection(ctx, r)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtFetchCDConnectionDetails, name, r.GetKind(), r.GetName())
		}

		cds.Merge(ComposedResourceState{
			ComposedResource:  ComposedResource{ResourceName: name},
			Resource:          r,
			ConnectionDetails: conn,
		})
	}

	return cds, nil
}

// FunctionIOObserved builds observed state for a FunctionIO from the XR and any
// existing composed resources. This reflects the observed state of the world
// before any Composition (P&T or function-based) has taken place.
func FunctionIOObserved(s *PTFCompositionState) (iov1alpha1.Observed, error) {
	raw, err := json.Marshal(s.Composite)
	if err != nil {
		return iov1alpha1.Observed{}, errors.Wrap(err, errMarshalXR)
	}

	rs := runtime.RawExtension{Raw: raw}
	econn := make([]iov1alpha1.ExplicitConnectionDetail, 0, len(s.ConnectionDetails))
	for n, v := range s.ConnectionDetails {
		econn = append(econn, iov1alpha1.ExplicitConnectionDetail{Name: n, Value: string(v)})
	}

	oxr := iov1alpha1.ObservedComposite{Resource: rs, ConnectionDetails: econn}

	ocds := make([]iov1alpha1.ObservedResource, 0, len(s.ComposedResources))
	for _, cd := range s.ComposedResources {

		raw, err := json.Marshal(cd.Resource)
		if err != nil {
			return iov1alpha1.Observed{}, errors.Wrap(err, errMarshalCD)
		}

		rs := runtime.RawExtension{Raw: raw}

		ecds := make([]iov1alpha1.ExplicitConnectionDetail, 0, len(cd.ConnectionDetails))
		for n, v := range cd.ConnectionDetails {
			ecds = append(ecds, iov1alpha1.ExplicitConnectionDetail{Name: n, Value: string(v)})
		}

		ocds = append(ocds, iov1alpha1.ObservedResource{
			Name:              cd.ResourceName,
			Resource:          rs,
			ConnectionDetails: ecds,
		})
	}

	return iov1alpha1.Observed{Composite: oxr, Resources: ocds}, nil
}

// An XRCDPatchAndTransformer runs a Composition's Patches & Transforms against
// both the XR and composed resources.
type XRCDPatchAndTransformer struct {
	composite Renderer
	composed  Renderer
}

// NewXRCDPatchAndTransformer returns a PatchAndTransformer that runs Patches
// and Transforms against both the XR and composed resources.
func NewXRCDPatchAndTransformer(composite, composed Renderer) *XRCDPatchAndTransformer {
	return &XRCDPatchAndTransformer{composite: composite, composed: composed}
}

// PatchAndTransform updates the supplied composition state by running all
// patches and transforms within the CompositionRequest.
func (pt *XRCDPatchAndTransformer) PatchAndTransform(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
	// Inline PatchSets before composing resources.
	ct, err := ComposedTemplates(req.Revision.Spec.PatchSets, req.Revision.Spec.Resources)
	if err != nil {
		return errors.Wrap(err, errInline)
	}

	// If we have an environment, run all environment patches before composing
	// resources.
	if req.Environment != nil && req.Revision.Spec.Environment != nil {
		for i, p := range req.Revision.Spec.Environment.Patches {
			if err := ApplyEnvironmentPatch(p, s.Composite, req.Environment); err != nil {
				return errors.Wrapf(err, errFmtPatchEnvironment, i)
			}
		}
	}

	// Render composite and composed resources using any P&T resource templates.
	// Note that we require templates to be named; a CompositionValidator should
	// enforce this.
	for i := range ct {
		t := ct[i]

		var r resource.Composed = composed.New()

		// Templates must be named. This is a requirement to use Composition
		// Functions and thus this Composer implementation.
		if cd, exists := s.ComposedResources[*t.Name]; exists {
			r = cd.Resource

			// Typically we'll patch from composed resource status to the XR so
			// we only want to render (i.e. patch) the XR from composed
			// resources that actually exist.
			if err := pt.composite.Render(ctx, s.Composite, r, t, req.Environment); err != nil {
				// TODO(negz): Why is it that an error rendering composed->XR is
				// terminal, but an error rendering XR->composed is not?
				return errors.Wrapf(err, errFmtRenderXR, *t.Name, r.GetObjectKind().GroupVersionKind().Kind, r.GetName())
			}
		}

		rerr := pt.composed.Render(ctx, s.Composite, r, t, req.Environment)
		if rerr != nil {
			// Failures to patch from XR->composed aren't terminal. It could be
			// that other resources need to patch the XR in order for the fields
			// this render wants to patch from to exist. Rather than returning
			// this error we just set Rendered = false in our state and return
			// a Warning event describing what happened.
			s.Events = append(s.Events, event.Warning(reasonCompose, errors.Wrapf(rerr, errFmtResourceName, *t.Name)))
		}

		s.ComposedResources.Merge(ComposedResourceState{
			ComposedResource:  ComposedResource{ResourceName: *t.Name},
			Resource:          r,
			Template:          &t,
			TemplateRenderErr: rerr,
		})
	}
	return nil
}

// FunctionIODesired builds the initial desired state for a FunctionIO from the XR
// and any existing or impending composed resources. This reflects the observed
// state of the world plus the initial desired state as built up by any P&T
// Composition that has taken place.
func FunctionIODesired(s *PTFCompositionState) (iov1alpha1.Desired, error) {
	raw, err := json.Marshal(s.Composite)
	if err != nil {
		return iov1alpha1.Desired{}, errors.Wrap(err, errMarshalXR)
	}

	rs := runtime.RawExtension{Raw: raw}
	econn := make([]iov1alpha1.ExplicitConnectionDetail, 0, len(s.ConnectionDetails))
	for n, v := range s.ConnectionDetails {
		econn = append(econn, iov1alpha1.ExplicitConnectionDetail{Name: n, Value: string(v)})
	}

	dxr := iov1alpha1.DesiredComposite{Resource: rs, ConnectionDetails: econn}

	dcds := make([]iov1alpha1.DesiredResource, 0, len(s.ComposedResources))
	for _, cd := range s.ComposedResources {
		if cd.Template == nil {
			// This composed resource isn't associated with a template. It must
			// be an existing resource that isn't desired by P&T Composition.
			// Perhaps it's only desired by the function pipeline, or was
			// desired by P&T but now isn't.
			continue
		}
		raw, err := json.Marshal(cd.Resource)
		if err != nil {
			return iov1alpha1.Desired{}, errors.Wrap(err, errMarshalCD)
		}
		dcds = append(dcds, iov1alpha1.DesiredResource{
			Name:     cd.ResourceName,
			Resource: runtime.RawExtension{Raw: raw},

			// TODO(negz): Should we include any connection details and
			// readiness checks from the P&T templates here? Doing so would
			// allow the composition function pipeline to alter them - i.e. to
			// remove details/checks. Currently the two are additive - we take
			// all the connection detail extraction configs and readiness checks
			// from the P&T process then append any from the function process.
		})
	}

	return iov1alpha1.Desired{Composite: dxr, Resources: dcds}, nil
}

// A ContainerFunctionRunner runs a containerized Composition Function.
type ContainerFunctionRunner interface {
	RunFunction(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error)
}

// A ContainerFunctionRunnerFn runs a containerized Composition Function.
type ContainerFunctionRunnerFn func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error)

// RunFunction runs a containerized Composition Function.
func (fn ContainerFunctionRunnerFn) RunFunction(ctx context.Context, fnio *iov1alpha1.FunctionIO, fnc *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
	return fn(ctx, fnio, fnc)
}

// A FunctionPipeline runs a pipeline of Composition Functions.
type FunctionPipeline struct {
	container ContainerFunctionRunner
}

// NewFunctionPipeline returns a FunctionPipeline that runs functions using the
// supplied ContainerFunctionRunner.
func NewFunctionPipeline(c ContainerFunctionRunner) *FunctionPipeline {
	return &FunctionPipeline{
		container: c,
	}
}

// RunFunctionPipeline runs a pipeline of Composition Functions.
func (p *FunctionPipeline) RunFunctionPipeline(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error { //nolint:gocyclo // Currently only at 12.
	r := make([]iov1alpha1.Result, 0)
	for _, fn := range req.Revision.Spec.Functions {
		switch fn.Type {
		case v1.FunctionTypeContainer:
			fnio, err := p.container.RunFunction(ctx, &iov1alpha1.FunctionIO{
				Config:   fn.Config,
				Observed: o,
				Desired:  d,
				Results:  r,
			}, fn.Container)
			if err != nil {
				return errors.Wrapf(err, errFmtRunFn, fn.Name)
			}
			// We require each function to pass through any results and desired
			// state from previous functions in the pipeline that they're
			// unconcerned with, as well as their own results and desired state.
			// We pass all functions the same observed state, since it should
			// represent the state before the function pipeline started.
			d = fnio.Desired
			r = fnio.Results
		default:
			return errors.Wrapf(errors.Errorf(errFmtUnsupportedFnType, fn.Type), errFmtRunFn, fn.Name)
		}
	}

	// Results of fatal severity stop the Composition process. Normal or warning
	// results are accumulated to be emitted as events by the Reconciler.
	for _, rs := range r {
		switch rs.Severity {
		case iov1alpha1.SeverityFatal:
			return errors.Wrap(errors.New(rs.Message), errFatalResult)
		case iov1alpha1.SeverityWarning:
			s.Events = append(s.Events, event.Warning(reasonCompose, errors.New(rs.Message)))
		case iov1alpha1.SeverityNormal:
			s.Events = append(s.Events, event.Normal(reasonCompose, rs.Message))
		}
	}

	u := &kunstructured.Unstructured{}
	if err := json.Unmarshal(d.Composite.Resource.Raw, u); err != nil {
		return errors.Wrap(err, errUnmarshalDesiredXR)
	}
	s.Composite = &composite.Unstructured{Unstructured: *u}

	s.ConnectionDetails = managed.ConnectionDetails{}
	for _, cd := range d.Composite.ConnectionDetails {
		s.ConnectionDetails[cd.Name] = []byte(cd.Value)
	}

	for _, dr := range d.Resources {
		cd, err := ParseDesiredResource(dr, s.Composite)
		if err != nil {
			return errors.Wrapf(err, errFmtParseDesiredCD, dr.Name)
		}

		s.ComposedResources.Merge(cd)
	}

	return nil
}

// DefaultCompositeFunctionRunner is a default runner for composite function
type DefaultCompositeFunctionRunner struct {
	Namespace      string
	ServiceAccount string
}

// RunFunction calls an external container function runner via gRPC.
func (r *DefaultCompositeFunctionRunner) RunFunction(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) { //nolint:gocyclo // Complexity is equal to 13 now

	in, err := yaml.Marshal(fnio)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalFnIO)
	}

	target := DefaultTarget
	if fn.Runner != nil && fn.Runner.Endpoint != nil {
		target = *fn.Runner.Endpoint
	}

	conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrap(err, errDialRunner)
	}

	k8schainOpts := k8schain.Options{}

	if r.Namespace != "" {
		k8schainOpts.Namespace = r.Namespace
	}

	if r.ServiceAccount != "" {
		k8schainOpts.ServiceAccountName = r.ServiceAccount
	}

	// pass all image pull secrets from composite function definition to keychain
	for _, ips := range fn.ImagePullSecrets {
		k8schainOpts.ImagePullSecrets = append(k8schainOpts.ImagePullSecrets, ips.Name)
	}

	keychain, err := k8schain.NewInCluster(ctx, k8schainOpts)
	// If we're not in a cluster keychain will be nil.
	if err != nil && !errors.Is(err, rest.ErrNotInCluster) {
		return nil, errors.Wrap(err, errNewKeychain)
	}

	pullConfig, err := ImagePullConfig(fn, keychain)
	if err != nil {
		return nil, errors.Wrap(err, errImgPullCfg)
	}
	req := &fnv1alpha1.RunFunctionRequest{
		Image:             fn.Image,
		Input:             in,
		ImagePullConfig:   pullConfig,
		RunFunctionConfig: RunFunctionConfig(fn),
	}
	rsp, err := fnv1alpha1.NewContainerizedFunctionRunnerServiceClient(conn).RunFunction(ctx, req)
	if err != nil {
		// TODO(negz): Parse any gRPC status codes.
		_ = conn.Close()
		return nil, errors.Wrap(err, errRunFnContainer)
	}

	if err := conn.Close(); err != nil {
		return nil, errors.Wrap(err, errCloseRunner)
	}

	// TODO(negz): Sanity check this FunctionIO to ensure the function returned
	// a valid response. Does it contain at least a desired Composite resource?
	out := &iov1alpha1.FunctionIO{}
	return out, errors.Wrap(yaml.Unmarshal(rsp.Output, out), errUnmarshalFnIO)
}

// ParseDesiredResource parses a (composed) DesiredResource from a FunctionIO.
// It adds some labels and annotations that are required for Crossplane to track
// the composed resources, but otherwise tries to be relatively unopinionated.
// It does not for example automatically generate a name for the composed
// resource; the Composition Function must do so.
func ParseDesiredResource(dr iov1alpha1.DesiredResource, owner resource.Object) (ComposedResourceState, error) {
	u := &kunstructured.Unstructured{}
	if err := json.Unmarshal(dr.Resource.Raw, u); err != nil {
		return ComposedResourceState{}, errors.Wrap(err, errUnmarshalDesiredCD)
	}

	r := &composed.Unstructured{Unstructured: *u}

	// Annotate the desired resource so we know which P&T resource template
	// and/or FunctionIO desired.resources entry it is associated with.
	SetCompositionResourceName(r, dr.Name)

	meta.AddLabels(r, map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: owner.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
		xcrd.LabelKeyClaimName:             owner.GetLabels()[xcrd.LabelKeyClaimName],
		xcrd.LabelKeyClaimNamespace:        owner.GetLabels()[xcrd.LabelKeyClaimNamespace],
	})

	// Ensure our XR is the controller of the resource.
	ref := meta.TypedReferenceTo(owner, owner.GetObjectKind().GroupVersionKind())
	if err := meta.AddControllerReference(r, meta.AsController(ref)); err != nil {
		return ComposedResourceState{}, errors.Wrap(err, errSetControllerRef)
	}

	cd := ComposedResourceState{
		ComposedResource: ComposedResource{ResourceName: dr.Name},
		Resource:         r,
		Desired:          &dr,
	}
	return cd, nil
}

// ImagePullConfig builds an ImagePullConfig for a FunctionIO.
func ImagePullConfig(fn *v1.ContainerFunction, keychain authn.Keychain) (*fnv1alpha1.ImagePullConfig, error) {
	cfg := &fnv1alpha1.ImagePullConfig{}

	if fn.ImagePullPolicy != nil {
		switch *fn.ImagePullPolicy {
		case corev1.PullAlways:
			cfg.PullPolicy = fnv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_ALWAYS
		case corev1.PullNever:
			cfg.PullPolicy = fnv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_NEVER
		case corev1.PullIfNotPresent:
			fallthrough
		default:
			cfg.PullPolicy = fnv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_IF_NOT_PRESENT
		}
	}
	if keychain == nil {
		return cfg, nil
	}

	tag, err := name.NewTag(fn.Image)
	if err != nil {
		return nil, err
	}
	auth, err := keychain.Resolve(tag)
	if err != nil {
		return nil, err
	}
	a, err := auth.Authorization()
	if err != nil {
		return nil, err
	}
	cfg.Auth = &fnv1alpha1.ImagePullAuth{
		Username:      a.Username,
		Password:      a.Password,
		Auth:          a.Auth,
		IdentityToken: a.IdentityToken,
		RegistryToken: a.RegistryToken,
	}
	return cfg, nil
}

// RunFunctionConfig builds a RunFunctionConfig for a FunctionIO.
func RunFunctionConfig(fn *v1.ContainerFunction) *fnv1alpha1.RunFunctionConfig {
	out := &fnv1alpha1.RunFunctionConfig{}
	if fn.Timeout != nil {
		out.Timeout = durationpb.New(fn.Timeout.Duration)
	}
	if fn.Resources != nil {
		out.Resources = &fnv1alpha1.ResourceConfig{}
		if fn.Resources.Limits != nil {
			out.Resources.Limits = &fnv1alpha1.ResourceLimits{}
			if fn.Resources.Limits.CPU != nil {
				out.Resources.Limits.Cpu = fn.Resources.Limits.CPU.String()
			}
			if fn.Resources.Limits.Memory != nil {
				out.Resources.Limits.Memory = fn.Resources.Limits.Memory.String()
			}
		}
	}
	if fn.Network != nil {
		out.Network = &fnv1alpha1.NetworkConfig{}
		if fn.Network.Policy != nil {
			switch *fn.Network.Policy {
			case v1.ContainerFunctionNetworkPolicyIsolated:
				out.Network.Policy = fnv1alpha1.NetworkPolicy_NETWORK_POLICY_ISOLATED
			case v1.ContainerFunctionNetworkPolicyRunner:
				out.Network.Policy = fnv1alpha1.NetworkPolicy_NETWORK_POLICY_RUNNER
			}
		}
	}
	return out
}

// An UndesiredComposedResourceDeleter deletes composed resources from the API
// server and from Composition state if their state doesn't include a FunctionIO
// desired resource. This indicates the composed resource either exists or was
// going to be created by P&T composition, but didn't survive the Composition
// Function pipeline.
type UndesiredComposedResourceDeleter struct {
	client client.Writer
}

// NewUndesiredComposedResourceDeleter returns a ComposedResourceDeleter that
// deletes undesired composed resources from both the API server and Composition
// state.
func NewUndesiredComposedResourceDeleter(c client.Writer) *UndesiredComposedResourceDeleter {
	return &UndesiredComposedResourceDeleter{client: c}
}

// DeleteComposedResources deletes any composed resource that didn't come out the other
// end of the Composition Function pipeline (i.e. that wasn't in the final
// desired state after running the pipeline). Composed resources are deleted
// from both the supposed composition state and from the API server.
func (d *UndesiredComposedResourceDeleter) DeleteComposedResources(ctx context.Context, s *PTFCompositionState) error {
	for name, cd := range s.ComposedResources {
		// We know this resource is still desired because we recorded its
		// desired state after running the FunctionIO pipeline. Don't garbage
		// collect it.
		if cd.Desired != nil {
			continue
		}

		// We don't desire this resource to exist; remove it from our state.
		delete(s.ComposedResources, name)

		// No need to garbage collect resources that don't exist.
		if !meta.WasCreated(cd.Resource) {
			continue
		}

		// We want to garbage collect this resource, but we don't control it.
		if c := metav1.GetControllerOf(cd.Resource); c == nil || c.UID != s.Composite.GetUID() {
			continue
		}

		if err := d.client.Delete(ctx, cd.Resource); resource.IgnoreNotFound(err) != nil {
			return errors.Wrapf(err, errFmtDeleteCD, cd.ResourceName, cd.Resource.GetObjectKind().GroupVersionKind().Kind, cd.Resource.GetName())
		}
	}

	return nil
}

// UpdateResourceRefs updates the supplied state to ensure the XR references all
// composed resources that exist or are pending creation.
func UpdateResourceRefs(s *PTFCompositionState) {
	refs := make([]corev1.ObjectReference, 0, len(s.ComposedResources))
	for _, cd := range s.ComposedResources {
		// Don't record references to resources that don't exist and that failed
		// to render. We won't apply (i.e. create) these resources this time
		// around, so there's no need to create dangling references to them.
		if !meta.WasCreated(cd.Resource) && cd.TemplateRenderErr != nil {
			continue
		}
		ref := meta.ReferenceTo(cd.Resource, cd.Resource.GetObjectKind().GroupVersionKind())
		refs = append(refs, *ref)
	}

	// We want to ensure our refs are stable.
	sort.Slice(refs, func(i, j int) bool {
		ri, rj := refs[i], refs[j]
		return ri.APIVersion+ri.Kind+ri.Name < rj.APIVersion+rj.Kind+rj.Name
	})

	s.Composite.SetResourceReferences(refs)
}

// A ComposedResourceObserverChain runs a slice of ComposedResourceObservers.
type ComposedResourceObserverChain []ComposedResourceObserver

// ObserveComposedResources runs the slice of ComposedResourceObservers.
func (o ComposedResourceObserverChain) ObserveComposedResources(ctx context.Context, s *PTFCompositionState) error {
	for _, cro := range o {
		if err := cro.ObserveComposedResources(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// A ReadinessObserver observes composed resource state and updates it to
// indicate whether each composed resource is ready per the readiness checks
// associated with each resource, which are derived from their P&T resource
// template and/or Composition Function desired state.
type ReadinessObserver struct {
	check ReadinessChecker
}

// NewReadinessObserver returns a ComposedResourceObserver that observes whether
// composed resources are ready.
func NewReadinessObserver(c ReadinessChecker) *ReadinessObserver {
	return &ReadinessObserver{check: c}
}

// ObserveComposedResources to determine their readiness.
func (o *ReadinessObserver) ObserveComposedResources(ctx context.Context, s *PTFCompositionState) error {
	for _, cd := range s.ComposedResources {
		rcfgs := append(ReadinessChecksFromTemplate(cd.Template), ReadinessChecksFromDesired(cd.Desired)...)
		ready, err := o.check.IsReady(ctx, cd.Resource, rcfgs...)
		if err != nil {
			return errors.Wrapf(err, errFmtReadiness, cd.ResourceName, cd.Resource.GetObjectKind().GroupVersionKind().Kind, cd.Resource.GetName())
		}

		s.ComposedResources.Merge(ComposedResourceState{
			ComposedResource: ComposedResource{
				ResourceName: cd.ResourceName,
				Ready:        ready,
			},
		})
	}

	return nil
}

// A ConnectionDetailsObserver extracts XR connection details from composed
// resource state. The details to extract are derived from each composed
// resource's P&T resource template and/or Composition Function desired state.
type ConnectionDetailsObserver struct {
	details ConnectionDetailsExtractor
}

// NewConnectionDetailsObserver returns a ComposedResourceObserver that observes
// composed resources in order to extract XR connection details.
func NewConnectionDetailsObserver(e ConnectionDetailsExtractor) *ConnectionDetailsObserver {
	return &ConnectionDetailsObserver{details: e}
}

// ObserveComposedResources to extract XR connection details.
func (o *ConnectionDetailsObserver) ObserveComposedResources(_ context.Context, s *PTFCompositionState) error {
	for _, cd := range s.ComposedResources {
		ecfgs := append(ExtractConfigsFromTemplate(cd.Template), ExtractConfigsFromDesired(cd.Desired)...)
		e, err := o.details.ExtractConnection(cd.Resource, cd.ConnectionDetails, ecfgs...)
		if err != nil {
			return errors.Wrapf(err, errFmtExtractConnectionDetails, cd.ResourceName, cd.Resource.GetObjectKind().GroupVersionKind().Kind, cd.Resource.GetName())
		}

		if s.ConnectionDetails == nil {
			s.ConnectionDetails = managed.ConnectionDetails{}
		}

		for key, val := range e {
			s.ConnectionDetails[key] = val
		}
	}

	return nil
}

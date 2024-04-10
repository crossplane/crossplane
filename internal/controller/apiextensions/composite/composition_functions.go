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
	"crypto/sha256"
	"fmt"
	"sort"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
)

// Error strings.
const (
	errFetchXRConnectionDetails = "cannot fetch composite resource connection details"
	errGetExistingCDs           = "cannot get existing composed resources"
	errBuildObserved            = "cannot build observed state for RunFunctionRequest"
	errGarbageCollectCDs        = "cannot garbage collect composed resources that are no longer desired"
	errApplyXRRefs              = "cannot update composite resource spec.resourceRefs"
	errApplyXRStatus            = "cannot apply composite resource status"
	errAnonymousCD              = "encountered composed resource without required \"" + AnnotationKeyCompositionResourceName + "\" annotation"
	errUnmarshalDesiredXRStatus = "cannot unmarshal desired composite resource status from RunFunctionResponse"
	errXRAsStruct               = "cannot encode composite resource to protocol buffer Struct well-known type"
	errEnvAsStruct              = "cannot encode environment to protocol buffer Struct well-known type"
	errStructFromUnstructured   = "cannot create Struct"

	errFmtApplyCD                    = "cannot apply composed resource %q"
	errFmtFetchCDConnectionDetails   = "cannot fetch connection details for composed resource %q (a %s named %s)"
	errFmtUnmarshalPipelineStepInput = "cannot unmarshal input for Composition pipeline step %q"
	errFmtRunPipelineStep            = "cannot run Composition pipeline step %q"
	errFmtDeleteCD                   = "cannot delete composed resource %q (a %s named %s)"
	errFmtUnmarshalDesiredCD         = "cannot unmarshal desired composed resource %q from RunFunctionResponse"
	errFmtCDAsStruct                 = "cannot encode composed resource %q to protocol buffer Struct well-known type"
	errFmtFatalResult                = "pipeline step %q returned a fatal result: %s"
)

// Server-side-apply field owners. We need two of these because it's possible
// an invocation of this controller will operate on the same resource in two
// different contexts. For example if an XR composes another XR we'll spin up
// two XR controllers. The 'parent' XR controller will treat the child XR as a
// composed resource, while the child XR controller will treat it as an XR. The
// controller owns different parts of the resource (i.e. has different fully
// specified intent) depending on the context.
const (
	// FieldOwnerXR owns the fields this controller mutates on composite
	// resources (XR).
	FieldOwnerXR = "apiextensions.crossplane.io/composite"

	// FieldOwnerComposedPrefix owns the fields this controller mutates on composed
	// resources.
	FieldOwnerComposedPrefix = "apiextensions.crossplane.io/composed"
)

const (
	// FunctionContextKeyEnvironment is used to store the Composition
	// Environment in the Function context.
	FunctionContextKeyEnvironment = "apiextensions.crossplane.io/environment"
)

// A FunctionComposer supports composing resources using a pipeline of
// Composition Functions. It ignores the P&T resources array.
type FunctionComposer struct {
	client    client.Client
	composite xr
	pipeline  FunctionRunner
}

type xr struct {
	NameGenerator
	managed.ConnectionDetailsFetcher
	ComposedResourceObserver
	ComposedResourceGarbageCollector
}

// A FunctionRunner runs a single Composition Function.
type FunctionRunner interface {
	// RunFunction runs the named Composition Function.
	RunFunction(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error)
}

// A FunctionRunnerFn is a function that can run a Composition Function.
type FunctionRunnerFn func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error)

// RunFunction runs the named Composition Function with the supplied request.
func (fn FunctionRunnerFn) RunFunction(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error) {
	return fn(ctx, name, req)
}

// A ComposedResourceObserver observes existing composed resources.
type ComposedResourceObserver interface {
	ObserveComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error)
}

// A ComposedResourceObserverFn observes existing composed resources.
type ComposedResourceObserverFn func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error)

// ObserveComposedResources observes existing composed resources.
func (fn ComposedResourceObserverFn) ObserveComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
	return fn(ctx, xr)
}

// A ComposedResourceGarbageCollector deletes observed composed resources that
// are no longer desired.
type ComposedResourceGarbageCollector interface {
	GarbageCollectComposedResources(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error
}

// A ComposedResourceGarbageCollectorFn deletes observed composed resources that
// are no longer desired.
type ComposedResourceGarbageCollectorFn func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error

// GarbageCollectComposedResources deletes observed composed resources that are
// no longer desired.
func (fn ComposedResourceGarbageCollectorFn) GarbageCollectComposedResources(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
	return fn(ctx, owner, observed, desired)
}

// A FunctionComposerOption is used to configure a FunctionComposer.
type FunctionComposerOption func(*FunctionComposer)

// WithCompositeConnectionDetailsFetcher configures how the FunctionComposer should
// get the composite resource's connection details.
func WithCompositeConnectionDetailsFetcher(f managed.ConnectionDetailsFetcher) FunctionComposerOption {
	return func(p *FunctionComposer) {
		p.composite.ConnectionDetailsFetcher = f
	}
}

// WithComposedResourceObserver configures how the FunctionComposer should get existing
// composed resources.
func WithComposedResourceObserver(g ComposedResourceObserver) FunctionComposerOption {
	return func(p *FunctionComposer) {
		p.composite.ComposedResourceObserver = g
	}
}

// WithComposedResourceGarbageCollector configures how the FunctionComposer should
// garbage collect undesired composed resources.
func WithComposedResourceGarbageCollector(d ComposedResourceGarbageCollector) FunctionComposerOption {
	return func(p *FunctionComposer) {
		p.composite.ComposedResourceGarbageCollector = d
	}
}

// NewFunctionComposer returns a new Composer that supports composing resources using
// both Patch and Transform (P&T) logic and a pipeline of Composition Functions.
func NewFunctionComposer(kube client.Client, r FunctionRunner, o ...FunctionComposerOption) *FunctionComposer {
	// TODO(negz): Can we avoid double-wrapping if the supplied client is
	// already wrapped? Or just do away with unstructured.NewClient completely?
	kube = unstructured.NewClient(kube)

	f := NewSecretConnectionDetailsFetcher(kube)

	c := &FunctionComposer{
		client: kube,

		composite: xr{
			ConnectionDetailsFetcher:         f,
			ComposedResourceObserver:         NewExistingComposedResourceObserver(kube, f),
			ComposedResourceGarbageCollector: NewDeletingComposedResourceGarbageCollector(kube),
			NameGenerator:                    NewAPINameGenerator(kube),
		},

		pipeline: r,
	}

	for _, fn := range o {
		fn(c)
	}

	return c
}

// Compose resources using the Functions pipeline.
func (c *FunctionComposer) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) { //nolint:gocyclo // We probably don't want any further abstraction for the sake of reduced complexity.
	// Observe our existing composed resources. We need to do this before we
	// render any P&T templates, so that we can make sure we use the same
	// composed resource names (as in, metadata.name) every time. We know what
	// composed resources exist because we read them from our XR's
	// spec.resourceRefs, so it's crucial that we never create a composed
	// resource without first persisting a reference to it.
	observed, err := c.composite.ObserveComposedResources(ctx, xr)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errGetExistingCDs)
	}

	// Build the initial observed and desired state to be passed to our
	// Composition Function pipeline. The observed state includes the XR and its
	// current (persisted) connection details, as well as any existing composed
	// resource and their current connection details. The desired state includes
	// only the XR and its connection details, which will initially be identical
	// to the observed state.
	xrConns, err := c.composite.FetchConnection(ctx, xr)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errFetchXRConnectionDetails)
	}
	o, err := AsState(xr, xrConns, observed)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errBuildObserved)
	}

	// The Function pipeline starts with empty desired state.
	d := &v1beta1.State{}

	events := []event.Event{}

	// The Function context starts empty...
	fctx := &structpb.Struct{Fields: map[string]*structpb.Value{}}

	// ...but we bootstrap it with the Composition environment, if there is one.
	if req.Environment != nil {
		e, err := AsStruct(req.Environment)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errEnvAsStruct)
		}
		fctx.Fields[FunctionContextKeyEnvironment] = structpb.NewStructValue(e)
	}

	// Run any Composition Functions in the pipeline. Each Function may mutate
	// the desired state returned by the last, and each Function may produce
	// results that will be emitted as events.
	for _, fn := range req.Revision.Spec.Pipeline {
		req := &v1beta1.RunFunctionRequest{Observed: o, Desired: d, Context: fctx}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				return CompositionResult{}, errors.Wrapf(err, errFmtUnmarshalPipelineStepInput, fn.Step)
			}
			req.Input = in
		}

		// TODO(negz): Generate a content-addressable tag for this request.
		// Perhaps using https://github.com/cerbos/protoc-gen-go-hashpb ?
		rsp, err := c.pipeline.RunFunction(ctx, fn.FunctionRef.Name, req)
		if err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtRunPipelineStep, fn.Step)
		}

		// Pass the desired state returned by this Function to the next one.
		d = rsp.GetDesired()

		// Pass the Function context returned by this Function to the next one.
		// We intentionally discard/ignore this after the last Function runs.
		fctx = rsp.GetContext()

		// Results of fatal severity stop the Composition process. Other results
		// are accumulated to be emitted as events by the Reconciler.
		for _, rs := range rsp.Results {
			switch rs.Severity {
			case v1beta1.Severity_SEVERITY_FATAL:
				return CompositionResult{}, errors.Errorf(errFmtFatalResult, fn.Step, rs.Message)
			case v1beta1.Severity_SEVERITY_WARNING:
				events = append(events, event.Warning(reasonCompose, errors.Errorf("Pipeline step %q: %s", fn.Step, rs.Message)))
			case v1beta1.Severity_SEVERITY_NORMAL:
				events = append(events, event.Normal(reasonCompose, fmt.Sprintf("Pipeline step %q: %s", fn.Step, rs.Message)))
			case v1beta1.Severity_SEVERITY_UNSPECIFIED:
				// We could hit this case if a Function was built against a newer
				// protobuf than this build of Crossplane, and the new protobuf
				// introduced a severity that we don't know about.
				events = append(events, event.Warning(reasonCompose, errors.Errorf("Pipeline step %q returned a result of unknown severity (assuming warning): %s", fn.Step, rs.Message)))
			}
		}
	}

	// Load our desired composed resources from the Function pipeline.
	desired := ComposedResourceStates{}
	for name, dr := range d.GetResources() {
		cd := composed.New()
		if err := FromStruct(cd, dr.GetResource()); err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtUnmarshalDesiredCD, name)
		}

		// If this desired resource state pertains to an existing composed
		// resource we want to maintain its name and namespace.
		or, ok := observed[ResourceName(name)]
		if ok {
			cd.SetNamespace(or.Resource.GetNamespace())
			cd.SetName(or.Resource.GetName())
		}

		// Set standard composed resource metadata that is derived from the XR.
		if err := RenderComposedResourceMetadata(cd, xr, ResourceName(name)); err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtRenderMetadata, name)
		}

		// Generate a name. We want to allocate this name before we actually
		// create the resource so that we can persist a resourceRef to it.
		// This ensures we don't leak composed resources - see
		// UpdateResourceRefs below.
		// Note: there is no guarantee this names stays free. But the chance
		// that it's taken before we create the object is low (there are 8
		// million names).
		if cd.GetName() == "" {
			if err := c.composite.GenerateName(ctx, cd); err != nil {
				return CompositionResult{}, errors.Wrapf(err, errFmtGenerateName, name)
			}
		}

		// TODO(negz): Should we try to automatically derive readiness if the
		// Function returns READY_UNSPECIFIED? Is it safe to assume that if the
		// Function doesn't have an opinion about readiness then we should look
		// for the Ready: True status condition?
		desired[ResourceName(name)] = ComposedResourceState{
			Resource:          cd,
			ConnectionDetails: dr.GetConnectionDetails(),
			Ready:             dr.Ready == v1beta1.Ready_READY_TRUE,
		}
	}

	// Garbage collect any observed resources that aren't part of our final
	// desired state. We must do this before we update the XR's resource
	// references to ensure that we don't forget and leak them if a delete
	// fails.
	if err := c.composite.GarbageCollectComposedResources(ctx, xr, observed, desired); err != nil {
		return CompositionResult{}, errors.Wrap(err, errGarbageCollectCDs)
	}

	// Record references to all desired composed resources. We need to do this
	// before we apply the composed resources in order to avoid potentially
	// leaking them. For example if we create three composed resources with
	// randomly generated names and hit an error applying the second one we need
	// to know that the first one (that _was_ created) exists next time we
	// reconcile the XR.
	refs := composite.New()
	refs.SetAPIVersion(xr.GetAPIVersion())
	refs.SetKind(xr.GetKind())
	refs.SetName(xr.GetName())
	UpdateResourceRefs(refs, desired)

	// Persist our updated composed resource references. We want this to be an
	// atomic replace of the entire array. Note that we're relying on the status
	// patch that immediately follows to load the latest version of uxr from the
	// API server.
	if err := c.client.Patch(ctx, refs, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerXR)); err != nil {
		// It's important we don't proceed if this fails, because we need to be
		// sure we've persisted our resource references before we create any new
		// composed resources below.
		return CompositionResult{}, errors.Wrap(err, errApplyXRRefs)
	}

	// Produce our array of resources to return to the Reconciler. The
	// Reconciler uses this array to determine whether the XR is ready.
	resources := make([]ComposedResource, 0, len(desired))

	// We apply all of our desired resources before we observe them in the loop
	// below. This ensures that issues observing and processing one composed
	// resource won't block the application of another.
	for name, cd := range desired {
		// We don't need any crossplane-runtime resource.Applicator style apply
		// options here because server-side apply takes care of everything.
		// Specifically it will merge rather than replace owner references (e.g.
		// for Usages), and will fail if we try to add a controller reference to
		// a resource that already has a different one.
		// NOTE(phisco): We need to set a field owner unique for each XR here,
		// this prevents multiple XRs composing the same resource to be
		// continuously alternated as controllers.
		if err := c.client.Patch(ctx, cd.Resource, client.Apply, client.ForceOwnership, client.FieldOwner(ComposedFieldOwnerName(xr))); err != nil {
			if kerrors.IsInvalid(err) {
				// We tried applying an invalid resource, we can't tell whether
				// this means the resource will never be valid or it will if we
				// run again the composition after some other resource is
				// created or updated successfully. So, we emit a warning event
				// and move on.
				// We mark the resource as not synced, so that once we get to
				// decide the XR's Synced condition, we can set it to false if
				// any of the resources didn't sync successfully.
				events = append(events, event.Warning(reasonCompose, errors.Wrapf(err, errFmtApplyCD, name)))
				// NOTE(phisco): here we behave differently w.r.t. the native
				// p&t composer, as we respect the readiness reported by
				// functions, while there we defaulted to also set ready false
				// in case of apply errors.
				resources = append(resources, ComposedResource{ResourceName: name, Ready: cd.Ready, Synced: false})
				continue
			}
			return CompositionResult{}, errors.Wrapf(err, errFmtApplyCD, name)
		}

		resources = append(resources, ComposedResource{ResourceName: name, Ready: cd.Ready, Synced: true})
	}

	// Our goal here is to patch our XR's status using server-side apply. We
	// want the resulting, patched object loaded into uxr. We need to pass in
	// only our "fully specified intent" - i.e. only the fields that we actually
	// care about. FromStruct will replace uxr's backing map[string]any with the
	// content of GetResource (i.e. the desired status). We then need to set its
	// GVK and name so that our client knows what resource to patch.
	v := xr.GetAPIVersion()
	k := xr.GetKind()
	n := xr.GetName()
	u := xr.GetUID()
	if err := FromStruct(xr, d.GetComposite().GetResource()); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUnmarshalDesiredXRStatus)
	}
	xr.SetAPIVersion(v)
	xr.SetKind(k)
	xr.SetName(n)
	xr.SetUID(u)

	// NOTE(phisco): Here we are fine using a hardcoded field owner as there is
	// no risk of conflict between different XRs.
	if err := c.client.Status().Patch(ctx, xr, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerXR)); err != nil {
		// Note(phisco): here we are fine with this error being terminal, as
		// there is no other resource to apply that might eventually resolve
		// this issue.
		return CompositionResult{}, errors.Wrap(err, errApplyXRStatus)
	}

	return CompositionResult{ConnectionDetails: d.GetComposite().GetConnectionDetails(), Composed: resources, Events: events}, nil
}

// ComposedFieldOwnerName generates a unique field owner name
// for a given Crossplane composite resource (XR). This uniqueness is crucial to
// prevent multiple XRs, which compose the same resource, from continuously
// alternating as controllers.
//
// The function generates a deterministic hash based on the XR's name and
// GroupKind (GK), ensuring consistency even during system restores. The hash
// does not include the XR's UID (as it's not deterministic), namespace (XRs
// don't have one), or version (to allow version changes without needing to
// update the field owner name).
//
// We decided to include the GK in the hash to prevent transferring ownership of
// composed resources across XRs with whole new GK, as that should not be
// supported without manual intervention.
//
// Given that field owner names are limited to 128 characters, the function
// truncates the hash to 32 characters. A longer hash was deemed unnecessary.
func ComposedFieldOwnerName(xr *composite.Unstructured) string {
	h := sha256.New()
	_, _ = h.Write([]byte(xr.GetName() + xr.GroupVersionKind().GroupKind().String()))
	return fmt.Sprintf("%s/%x", FieldOwnerComposedPrefix, h.Sum(nil))
}

// An ExistingComposedResourceObserver uses an XR's resource references to load
// any existing composed resources from the API server. It also loads their
// connection details.
type ExistingComposedResourceObserver struct {
	resource client.Reader
	details  managed.ConnectionDetailsFetcher
}

// NewExistingComposedResourceObserver returns a ComposedResourceGetter that
// fetches an XR's existing composed resources.
func NewExistingComposedResourceObserver(c client.Reader, f managed.ConnectionDetailsFetcher) *ExistingComposedResourceObserver {
	return &ExistingComposedResourceObserver{resource: c, details: f}
}

// ObserveComposedResources begins building composed resource state by
// fetching any existing composed resources referenced by the supplied composite
// resource, as well as their connection details.
func (g *ExistingComposedResourceObserver) ObserveComposedResources(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
	ors := ComposedResourceStates{}

	for _, ref := range xr.GetResourceReferences() {
		// The PTComposer writes references to resources that it didn't actually
		// render or create. It has to create these placeholder refs because it
		// supports anonymous (unnamed) resource templates; it needs to be able
		// associate entries a Composition's spec.resources array with entries
		// in an XR's spec.resourceRefs array by their index. These references
		// won't have a name - we won't be able to get them because they don't
		// reference a resource that actually exists. We make this check to
		// cover the (hopefully tiny) edge case where an XR has switched from
		// P&T Composition to Functions, but has one or more composed resources
		// that have been failing to render.
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

		ors[name] = ComposedResourceState{Resource: r, ConnectionDetails: conn}
	}

	return ors, nil
}

// AsState builds state for a RunFunctionRequest from the XR and composed
// resources.
func AsState(xr resource.Composite, xc managed.ConnectionDetails, rs ComposedResourceStates) (*v1beta1.State, error) {
	r, err := AsStruct(xr)
	if err != nil {
		return nil, errors.Wrap(err, errXRAsStruct)
	}

	oxr := &v1beta1.Resource{Resource: r, ConnectionDetails: xc}

	ocds := make(map[string]*v1beta1.Resource)
	for name, or := range rs {
		r, err := AsStruct(or.Resource)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtCDAsStruct, name)
		}

		ocds[string(name)] = &v1beta1.Resource{Resource: r, ConnectionDetails: or.ConnectionDetails}
	}

	return &v1beta1.State{Composite: oxr, Resources: ocds}, nil
}

// AsStruct converts the supplied object to a protocol buffer Struct well-known
// type.
func AsStruct(o runtime.Object) (*structpb.Struct, error) {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(*kunstructured.Unstructured); ok {
		s, err := structpb.NewStruct(u.Object)
		return s, errors.Wrap(err, errStructFromUnstructured)
	}

	// If the supplied object wraps *Unstructured we don't need to round-trip.
	if w, ok := o.(unstructured.Wrapper); ok {
		s, err := structpb.NewStruct(w.GetUnstructured().Object)
		return s, errors.Wrap(err, errStructFromUnstructured)
	}

	// Fall back to a JSON round-trip.
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalJSON)
	}

	s := &structpb.Struct{}
	return s, errors.Wrap(s.UnmarshalJSON(b), errUnmarshalJSON)
}

// FromStruct populates the supplied object with content loaded from the Struct.
func FromStruct(o client.Object, s *structpb.Struct) error {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(*kunstructured.Unstructured); ok {
		u.Object = s.AsMap()
		return nil
	}

	// If the supplied object wraps *Unstructured we don't need to round-trip.
	if w, ok := o.(unstructured.Wrapper); ok {
		w.GetUnstructured().Object = s.AsMap()
		return nil
	}

	// Fall back to a JSON round-trip.
	b, err := protojson.Marshal(s)
	if err != nil {
		return errors.Wrap(err, errMarshalProtoStruct)
	}

	return errors.Wrap(json.Unmarshal(b, o), errUnmarshalJSON)
}

// An DeletingComposedResourceGarbageCollector deletes undesired composed resources from
// the API server.
type DeletingComposedResourceGarbageCollector struct {
	client client.Writer
}

// NewDeletingComposedResourceGarbageCollector returns a ComposedResourceDeleter that
// deletes undesired composed resources from the API server.
func NewDeletingComposedResourceGarbageCollector(c client.Writer) *DeletingComposedResourceGarbageCollector {
	return &DeletingComposedResourceGarbageCollector{client: c}
}

// GarbageCollectComposedResources deletes any composed resource that didn't
// come out the other end of the Composition Function pipeline (i.e. that wasn't
// in the final desired state after running the pipeline) from the API server.
func (d *DeletingComposedResourceGarbageCollector) GarbageCollectComposedResources(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
	del := ComposedResourceStates{}
	for name, cd := range observed {
		if _, ok := desired[name]; !ok {
			del[name] = cd
		}
	}

	for name, cd := range del {
		// We want to garbage collect this resource, but we don't control it.
		if c := metav1.GetControllerOf(cd.Resource); c == nil || c.UID != owner.GetUID() {
			continue
		}

		if err := d.client.Delete(ctx, cd.Resource); resource.IgnoreNotFound(err) != nil {
			return errors.Wrapf(err, errFmtDeleteCD, name, cd.Resource.GetObjectKind().GroupVersionKind().Kind, cd.Resource.GetName())
		}
	}

	return nil
}

// UpdateResourceRefs updates the supplied state to ensure the XR references all
// composed resources that exist or are pending creation.
func UpdateResourceRefs(xr resource.ComposedResourcesReferencer, desired ComposedResourceStates) {
	refs := make([]corev1.ObjectReference, 0, len(desired))
	for _, dr := range desired {
		ref := meta.ReferenceTo(dr.Resource, dr.Resource.GetObjectKind().GroupVersionKind())
		refs = append(refs, *ref)
	}

	// We want to ensure our refs are stable.
	sort.Slice(refs, func(i, j int) bool {
		ri, rj := refs[i], refs[j]
		return ri.APIVersion+ri.Kind+ri.Name < rj.APIVersion+rj.Kind+rj.Name
	})

	xr.SetResourceReferences(refs)
}

/*
Copyright 2020 The Crossplane Authors.

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

package composite

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/usage"
	"github.com/crossplane/crossplane/internal/names"
)

// Error strings.
const (
	errGetComposed   = "cannot get composed resource"
	errGCComposed    = "cannot garbage collect composed resource"
	errApplyComposed = "cannot apply composed resource"
	errFetchDetails  = "cannot fetch connection details"
	errInline        = "cannot inline Composition patch sets"

	errFmtPatchEnvironment           = "cannot apply environment patch at index %d"
	errFmtParseBase                  = "cannot parse base template of composed resource %q"
	errFmtRenderFromCompositePatches = "cannot render FromComposite or environment patches for composed resource %q"
	errFmtRenderToCompositePatches   = "cannot render ToComposite patches for composed resource %q"
	errFmtRenderMetadata             = "cannot render metadata for composed resource %q"
	errFmtGenerateName               = "cannot generate a name for composed resource %q"
	errFmtExtractDetails             = "cannot extract composite resource connection details from composed resource %q"
	errFmtCheckReadiness             = "cannot check whether composed resource %q is ready"
)

// TODO(negz): Move P&T Composition logic into its own package?

// A PTComposerOption is used to configure a PTComposer.
type PTComposerOption func(*PTComposer)

// WithTemplateAssociator configures how a PatchAndTransformComposer associates
// templates with extant composed resources.
func WithTemplateAssociator(a CompositionTemplateAssociator) PTComposerOption {
	return func(c *PTComposer) {
		c.composition = a
	}
}

// WithComposedNameGenerator configures how the PTComposer should generate names
// for unnamed composed resources.
func WithComposedNameGenerator(r names.NameGenerator) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.NameGenerator = r
	}
}

// WithComposedReadinessChecker configures how a PatchAndTransformComposer
// checks composed resource readiness.
func WithComposedReadinessChecker(r ReadinessChecker) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ReadinessChecker = r
	}
}

// WithComposedConnectionDetailsFetcher configures how a
// PatchAndTransformComposer fetches composed resource connection details.
func WithComposedConnectionDetailsFetcher(f managed.ConnectionDetailsFetcher) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ConnectionDetailsFetcher = f
	}
}

// WithComposedConnectionDetailsExtractor configures how a
// PatchAndTransformComposer extracts XR connection details from a composed
// resource.
func WithComposedConnectionDetailsExtractor(e ConnectionDetailsExtractor) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ConnectionDetailsExtractor = e
	}
}

type composedResource struct {
	names.NameGenerator
	managed.ConnectionDetailsFetcher
	ConnectionDetailsExtractor
	ReadinessChecker
}

// A PTComposer composes resources using Patch and Transform (P&T) Composition.
// It uses a Composition's 'resources' array, which consist of 'base' resources
// along with a series of patches and transforms. It does not support Functions
// - any entries in the functions array are ignored.
type PTComposer struct {
	client resource.ClientApplicator

	composition CompositionTemplateAssociator
	composed    composedResource
}

// NewPTComposer returns a Composer that composes resources using Patch and
// Transform (P&T) Composition - a Composition's bases, patches, and transforms.
func NewPTComposer(kube client.Client, o ...PTComposerOption) *PTComposer {
	// TODO(negz): Can we avoid double-wrapping if the supplied client is
	// already wrapped? Or just do away with unstructured.NewClient completely?
	kube = unstructured.NewClient(kube)

	c := &PTComposer{
		client: resource.ClientApplicator{Client: kube, Applicator: resource.NewAPIPatchingApplicator(kube)},

		composition: NewGarbageCollectingAssociator(kube),
		composed: composedResource{
			NameGenerator:              names.NewNameGenerator(kube),
			ReadinessChecker:           ReadinessCheckerFn(IsReady),
			ConnectionDetailsFetcher:   NewSecretConnectionDetailsFetcher(kube),
			ConnectionDetailsExtractor: ConnectionDetailsExtractorFn(ExtractConnectionDetails),
		},
	}

	for _, fn := range o {
		fn(c)
	}

	return c
}

// Compose resources using the bases, patches, and transforms specified by the
// supplied Composition. This reconciler supports only Patch & Transform
// Composition (not the Function pipeline). It does this in roughly four steps:
//
//  1. Figure out which templates are associated with which existing composed
//     resources, if any.
//  2. Render from those templates into new or existing composed resources.
//  3. Apply all composed resources that rendered successfully.
//  4. Observe the readiness and connection details of all composed resources
//     that rendered successfully.
func (c *PTComposer) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) { //nolint:gocognit // Breaking this up doesn't seem worth yet more layers of abstraction.
	// Inline PatchSets before composing resources.
	ct, err := ComposedTemplates(req.Revision.Spec.PatchSets, req.Revision.Spec.Resources)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errInline)
	}

	// Figure out which templates are associated with which existing composed
	// resources. This results in an array of templates associated with an array
	// of entries in the XR's spec.resourceRefs array. If we're using a
	// Composition with anonymous resource templates they'll be associated
	// strictly by order. If we're using a Composition with named resource
	// templates we'll be able to instead read the template name annotation from
	// the composed resources to make the annotation.
	tas, err := c.composition.AssociateTemplates(ctx, xr, ct)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errAssociate)
	}

	// If we have an environment, run all environment patches before composing
	// resources.
	if req.Environment != nil && req.Revision.Spec.Environment != nil {
		for i, p := range req.Revision.Spec.Environment.Patches {
			if err := ApplyEnvironmentPatch(p, xr, req.Environment); err != nil {
				return CompositionResult{}, errors.Wrapf(err, errFmtPatchEnvironment, i)
			}
		}
	}

	events := make([]event.Event, 0)

	// We optimistically render all composed resources that we are able to with
	// the expectation that any that we fail to render will subsequently have
	// their error corrected by manual intervention or propagation of a required
	// input. Errors are recorded, but not considered fatal to the composition
	// process.
	refs := make([]corev1.ObjectReference, len(tas))
	cds := make([]resource.Composed, len(tas))
	for i := range tas {
		ta := tas[i]

		// If this resource is anonymous its "name" is just its index.
		name := ptr.Deref(ta.Template.Name, fmt.Sprintf("resource %d", i+1))
		r := composed.New(composed.FromReference(ta.Reference))

		if err := RenderFromJSON(r, ta.Template.Base.Raw); err != nil {
			// We consider this a terminal error, since it indicates a broken
			// CompositionRevision that will never be valid.
			return CompositionResult{}, errors.Wrapf(err, errFmtParseBase, name)
		}

		// Failures to patch aren't terminal - we just emit a warning event and
		// move on. This is because patches often fail because other patches
		// need to happen first in order for them to succeed. If we returned an
		// error when a patch failed we might never reach the patch that would
		// unblock it.

		rendered := true
		if err := RenderFromCompositeAndEnvironmentPatches(r, xr, req.Environment, ta.Template.Patches); err != nil {
			events = append(events, event.Warning(reasonCompose, errors.Wrapf(err, errFmtRenderFromCompositePatches, name)))
			rendered = false
		}

		if err := RenderComposedResourceMetadata(r, xr, ResourceName(ptr.Deref(ta.Template.Name, ""))); err != nil {
			events = append(events, event.Warning(reasonCompose, errors.Wrapf(err, errFmtRenderMetadata, name)))
			rendered = false
		}

		if err := c.composed.GenerateName(ctx, r); err != nil {
			events = append(events, event.Warning(reasonCompose, errors.Wrapf(err, errFmtGenerateName, name)))
			rendered = false
		}

		// We record a reference even if we didn't render the resource because
		// if it already exists we don't want to drop our reference to it (and
		// thus not know about it next reconcile). If we're using anonymous
		// resource templates we also need to record a reference even if it's
		// empty, so that our XR's spec.resourceRefs remains the same length and
		// order as our CompositionRevisions's array of templates.
		refs[i] = *meta.ReferenceTo(r, r.GetObjectKind().GroupVersionKind())

		// We only need the composed resource if it rendered correctly.
		if rendered {
			cds[i] = r
		}
	}

	// We persist references to our composed resources before we create
	// them. This way we can render composed resources with
	// non-deterministic names, and also potentially recover from any errors
	// we encounter while applying composed resources without leaking them.
	xr.SetResourceReferences(refs)
	if err := c.client.Update(ctx, xr); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	// We apply all of our composed resources before we observe them in the
	// loop below. This ensures that issues observing and processing one
	// composed resource won't block the application of another.
	for i := range tas {
		t := tas[i].Template
		cd := cds[i]

		// If we were unable to render the composed resource we should not try
		// and apply it. The risk of doing so is that we successfully apply a
		// partially-rendered composed resource that we can't later fix (e.g.
		// due to an immutable field).
		if cd == nil {
			continue
		}

		o := []resource.ApplyOption{resource.MustBeControllableBy(xr.GetUID()), usage.RespectOwnerRefs()}
		o = append(o, mergeOptions(filterPatches(t.Patches, patchTypesFromXR()...))...)
		if err := c.client.Apply(ctx, cd, o...); err != nil {
			if kerrors.IsInvalid(err) {
				// We tried applying an invalid resource, we can't tell whether
				// this means the resource will never be valid or it will if we
				// run again the composition after some other resource is
				// created or updated successfully. So, we emit a warning event
				// and move on.
				events = append(events, event.Warning(reasonCompose, errors.Wrap(err, errApplyComposed)))
				// We unset the cd here so that we don't try to observe it
				// later. This will also mean we report it as not ready and not
				// synced. Resulting in the XR being reported as not ready nor
				// synced too.
				cds[i] = nil
				continue
			}

			// TODO(negz): Include the template name (if any) in this error.
			// Including the rendered resource's kind may help too (e.g. if the
			// template is anonymous).
			return CompositionResult{}, errors.Wrap(err, errApplyComposed)
		}
	}

	// Produce our array of resources to return to the Reconciler. The
	// Reconciler uses this array to determine whether the XR is ready. This
	// means it's important that we return a resources resource for every entry
	// in tas - i.e. a resources resource for every resource template.
	resources := make([]ComposedResource, len(tas))
	xrConnDetails := managed.ConnectionDetails{}
	for i := range tas {
		t := tas[i].Template
		cd := cds[i]

		// If this resource is anonymous its "name" is just its index within the
		// array of composed resource templates.
		name := ResourceName(ptr.Deref(t.Name, fmt.Sprintf("resource %d", i+1)))

		// If we were unable to render the composed resource we should not try
		// to observe it. We still want to return it to the Reconciler so that
		// it knows that this desired composed resource is not ready.
		if cd == nil {
			resources[i] = ComposedResource{ResourceName: name, Synced: false, Ready: false}
			continue
		}

		if err := RenderToCompositePatches(xr, cd, t.Patches); err != nil {
			// Failures to render ToComposite patches are terminal because this
			// indicates a Required ToCompositeFieldPath patch failed; i.e. the
			// composite was _required_ to be patched, but wasn't.
			return CompositionResult{}, errors.Wrapf(err, errFmtRenderToCompositePatches, name)
		}

		cdConnDetails, err := c.composed.FetchConnection(ctx, cd)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errFetchDetails)
		}

		extracted, err := c.composed.ExtractConnection(cd, cdConnDetails, ExtractConfigsFromComposedTemplate(&t)...)
		if err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtExtractDetails, name)
		}

		for key, val := range extracted {
			xrConnDetails[key] = val
		}

		ready, err := c.composed.IsReady(ctx, cd, ReadinessChecksFromComposedTemplate(&t)...)
		if err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtCheckReadiness, name)
		}

		resources[i] = ComposedResource{ResourceName: name, Ready: ready, Synced: true}
	}

	// Call Apply so that we do not just replace fields on existing XR but
	// merge fields for which a merge configuration has been specified. For
	// fields for which a merge configuration does not exist, the behavior
	// will be a replace from copy. We pass a deepcopy because the Apply
	// method doesn't update status, but calling Apply resets any pending
	// status changes.
	//
	// Unless this Apply is a no-op it will cause the XR's resource version to
	// be incremented. Our original copy of the XR (cr) will still have the old
	// resource version, so subsequent attempts to update it or its status will
	// be rejected by the API server. This will trigger an immediate requeue,
	// and we'll proceed to update the status as soon as there are no changes to
	// be made to the spec.
	objCopy := xr.DeepCopy()
	if err := c.client.Apply(ctx, objCopy, mergeOptions(toXRPatchesFromTAs(tas))...); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	return CompositionResult{ConnectionDetails: xrConnDetails, Composed: resources, Events: events}, nil
}

// toXRPatchesFromTAs selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite).
func toXRPatchesFromTAs(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}

// filterPatches selects patches whose type belong to the list onlyTypes.
func filterPatches(pas []v1.Patch, onlyTypes ...v1.PatchType) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(pas))
	include := make(map[v1.PatchType]bool)
	for _, t := range onlyTypes {
		include[t] = true
	}
	for _, p := range pas {
		if include[p.Type] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// A TemplateAssociation associates a composed resource template with a composed
// resource. If no such resource exists the reference will be empty.
type TemplateAssociation struct {
	Template  v1.ComposedTemplate
	Reference corev1.ObjectReference
}

// AssociateByOrder associates the supplied templates with the supplied resource
// references by order; i.e. by assuming template n corresponds to reference n.
// The returned array will always be of the same length as the supplied array of
// templates. Any additional references will be truncated.
func AssociateByOrder(t []v1.ComposedTemplate, r []corev1.ObjectReference) []TemplateAssociation {
	a := make([]TemplateAssociation, len(t))
	for i := range t {
		a[i] = TemplateAssociation{Template: t[i]}
	}

	j := len(t)
	if len(r) < j {
		j = len(r)
	}

	for i := range j {
		a[i].Reference = r[i]
	}

	return a
}

// A CompositionTemplateAssociator returns an array of template associations.
type CompositionTemplateAssociator interface {
	AssociateTemplates(ctx context.Context, xr resource.Composite, cts []v1.ComposedTemplate) ([]TemplateAssociation, error)
}

// A CompositionTemplateAssociatorFn returns an array of template associations.
type CompositionTemplateAssociatorFn func(context.Context, resource.Composite, []v1.ComposedTemplate) ([]TemplateAssociation, error)

// AssociateTemplates with composed resources.
func (fn CompositionTemplateAssociatorFn) AssociateTemplates(ctx context.Context, cr resource.Composite, ct []v1.ComposedTemplate) ([]TemplateAssociation, error) {
	return fn(ctx, cr, ct)
}

// A GarbageCollectingAssociator associates a Composition's resource templates
// with (references to) composed resources. It tries to associate them by
// checking the template name annotation of each referenced resource. If any
// template or existing composed resource can't be associated by name it falls
// back to associating them by order. If it encounters a referenced resource
// that corresponds to a non-existent template the resource will be garbage
// collected (i.e. deleted).
type GarbageCollectingAssociator struct {
	client client.Client
}

// NewGarbageCollectingAssociator returns a CompositionTemplateAssociator that
// may garbage collect composed resources.
func NewGarbageCollectingAssociator(c client.Client) *GarbageCollectingAssociator {
	return &GarbageCollectingAssociator{client: c}
}

// AssociateTemplates with composed resources.
func (a *GarbageCollectingAssociator) AssociateTemplates(ctx context.Context, cr resource.Composite, ct []v1.ComposedTemplate) ([]TemplateAssociation, error) {
	templates := map[ResourceName]int{}
	for i, t := range ct {
		if t.Name == nil {
			// If our templates aren't named we fall back to assuming that the
			// existing resource reference array (if any) already matches the
			// order of our resource template array.
			return AssociateByOrder(ct, cr.GetResourceReferences()), nil
		}
		templates[ResourceName(*t.Name)] = i
	}

	tas := make([]TemplateAssociation, len(ct))
	for i := range ct {
		tas[i] = TemplateAssociation{Template: ct[i]}
	}

	for _, ref := range cr.GetResourceReferences() {
		// If reference does not have a name then we haven't rendered it yet.
		if ref.Name == "" {
			continue
		}
		cd := composed.New(composed.FromReference(ref))
		nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		err := a.client.Get(ctx, nn, cd)

		// We believe we created this resource, but it no longer exists.
		if kerrors.IsNotFound(err) {
			continue
		}

		if err != nil {
			return nil, errors.Wrap(err, errGetComposed)
		}

		name := GetCompositionResourceName(cd)
		if name == "" {
			// All of our templates are named, but this existing composed
			// resource is not associated with a named template. It's likely
			// that our Composition was just migrated from anonymous to named
			// templates. We fall back to assuming that the existing resource
			// reference array already matches the order of our resource
			// template array. Existing composed resources should be annotated
			// at render time with the name of the template used to create them.
			return AssociateByOrder(ct, cr.GetResourceReferences()), nil
		}

		// Inject the reference to this existing resource into the references
		// array position that matches the templates array position of the
		// template the resource corresponds to.
		if i, ok := templates[name]; ok {
			tas[i].Reference = ref
			continue
		}

		// We want to garbage collect this resource, but we don't control it.
		if c := metav1.GetControllerOf(cd); c == nil || c.UID != cr.GetUID() {
			continue
		}

		// This existing resource does not correspond to an extant template. It
		// should be garbage collected.
		if err := a.client.Delete(ctx, cd); resource.IgnoreNotFound(err) != nil {
			return nil, errors.Wrap(err, errGCComposed)
		}
	}

	return tas, nil
}

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               corev1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}

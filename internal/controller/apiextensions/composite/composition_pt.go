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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	env "github.com/crossplane/crossplane/internal/controller/apiextensions/composite/environment"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings
const (
	errGetComposed      = "cannot get composed resource"
	errGCComposed       = "cannot garbage collect composed resource"
	errApply            = "cannot apply composed resource"
	errFetchDetails     = "cannot fetch connection details"
	errExtractDetails   = "cannot extract composite resource connection details from composed resource"
	errReadiness        = "cannot check whether composed resource is ready"
	errUnmarshal        = "cannot unmarshal base template"
	errGetSecret        = "cannot get connection secret of composed resource"
	errNamePrefix       = "name prefix is not found in labels"
	errKindChanged      = "cannot change the kind of an existing composed resource"
	errName             = "cannot use dry-run create to name composed resource"
	errInline           = "cannot inline Composition patch sets"
	errRenderCR         = "cannot render composite resource"
	errSetControllerRef = "cannot set controller reference"

	errFmtResourceName = "composed resource %q"
	errFmtPatch        = "cannot apply the patch at index %d"
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

// WithCompositeRenderer configures how a PatchAndTransformComposer renders the
// composite resource.
func WithCompositeRenderer(r Renderer) PTComposerOption {
	return func(c *PTComposer) {
		c.composite = r
	}
}

// WithComposedRenderer configures how a PatchAndTransformComposer renders
// composed resources.
func WithComposedRenderer(r Renderer) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.Renderer = r
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
	Renderer
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

	composite   Renderer
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

		// TODO(negz): Once Composition Functions are GA this Composer will only
		// need to handle legacy Compositions that use anonymous templates. This
		// means we will be able to delete the GarbageCollectingAssociator and
		// just use AssociateByOrder. Compositions with named templates will be
		// handled by the PTFComposer.
		composite:   RendererFn(RenderComposite),
		composition: NewGarbageCollectingAssociator(kube),
		composed: composedResource{
			Renderer:                   NewAPIDryRunRenderer(kube),
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
// supplied Composition.
func (c *PTComposer) Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) { //nolint:gocyclo // Breaking this up doesn't seem worth yet more layers of abstraction.
	// Inline PatchSets before composing resources.
	ct, err := ComposedTemplates(req.Revision.Spec.PatchSets, req.Revision.Spec.Resources)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errInline)
	}

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
	cds := make([]ComposedResourceState, len(tas))
	for i := range tas {
		ta := tas[i]

		// If this resource is anonymous its "name" is just its index.
		name := pointer.StringDeref(ta.Template.Name, strconv.Itoa(i))
		r := composed.New(composed.FromReference(ta.Reference))

		rerr := c.composed.Render(ctx, xr, r, ta.Template, req.Environment)
		if rerr != nil {
			events = append(events, event.Warning(reasonCompose, errors.Wrapf(rerr, errFmtResourceName, name)))
		}

		cds[i] = ComposedResourceState{
			ComposedResource:  ComposedResource{ResourceName: name},
			TemplateRenderErr: rerr,
			Template:          &ta.Template,
			Resource:          r,
		}
		refs[i] = *meta.ReferenceTo(r, r.GetObjectKind().GroupVersionKind())
	}

	// We persist references to our composed resources before we create
	// them. This way we can render composed resources with
	// non-deterministic names, and also potentially recover from any errors
	// we encounter while applying composed resources without leaking them.
	xr.SetResourceReferences(refs)
	if err := c.client.Update(ctx, xr); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	// We apply all of our composed resources before we observe them and update
	// in the loop below. This ensures that issues observing and processing one
	// composed resource won't block the application of another.
	for _, cd := range cds {
		// If we were unable to render the composed resource we should not try
		// and apply it.
		if cd.TemplateRenderErr != nil {
			continue
		}
		o := []resource.ApplyOption{resource.MustBeControllableBy(xr.GetUID())}
		o = append(o, mergeOptions(filterPatches(cd.Template.Patches, patchTypesFromXR()...))...)
		if err := c.client.Apply(ctx, cd.Resource, o...); err != nil {
			return CompositionResult{}, errors.Wrap(err, errApply)
		}
	}

	conn := managed.ConnectionDetails{}
	for i := range cds {
		// If we were unable to render the composed resource we should not try
		// to observe it.
		if cds[i].TemplateRenderErr != nil {
			continue
		}

		if err := c.composite.Render(ctx, xr, cds[i].Resource, *cds[i].Template, req.Environment); err != nil {
			return CompositionResult{}, errors.Wrap(err, errRenderCR)
		}

		cds[i].ConnectionDetails, err = c.composed.FetchConnection(ctx, cds[i].Resource)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errFetchDetails)
		}

		e, err := c.composed.ExtractConnection(cds[i].Resource, cds[i].ConnectionDetails, ExtractConfigsFromTemplate(cds[i].Template)...)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errExtractDetails)
		}

		for key, val := range e {
			conn[key] = val
		}

		cds[i].Ready, err = c.composed.IsReady(ctx, cds[i].Resource, ReadinessChecksFromTemplate(cds[i].Template)...)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errReadiness)
		}
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
	objCopy := xr.DeepCopyObject().(client.Object)
	if err := c.client.Apply(ctx, objCopy, mergeOptions(toXRPatchesFromTAs(tas))...); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	out := make([]ComposedResource, len(cds))
	for i := range cds {
		out[i] = cds[i].ComposedResource
	}

	return CompositionResult{ConnectionDetails: conn, Composed: out, Events: events}, nil
}

// toXRPatchesFromTAs selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite)
func toXRPatchesFromTAs(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}

// filterPatches selects patches whose type belong to the list onlyTypes
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

	for i := 0; i < j; i++ {
		a[i].Reference = r[i]
	}

	return a
}

// A CompositionTemplateAssociator returns an array of template associations.
type CompositionTemplateAssociator interface {
	AssociateTemplates(context.Context, resource.Composite, []v1.ComposedTemplate) ([]TemplateAssociation, error)
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
func (a *GarbageCollectingAssociator) AssociateTemplates(ctx context.Context, cr resource.Composite, ct []v1.ComposedTemplate) ([]TemplateAssociation, error) { //nolint:gocyclo // Only slightly over (13).
	templates := map[string]int{}
	for i, t := range ct {
		if t.Name == nil {
			// If our templates aren't named we fall back to assuming that the
			// existing resource reference array (if any) already matches the
			// order of our resource template array.
			return AssociateByOrder(ct, cr.GetResourceReferences()), nil
		}
		templates[*t.Name] = i
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

		// TODO(negz): Below should be || not &&. If the controller ref is nil
		// we don't control the resource and shouldn't delete it.

		// We want to garbage collect this resource, but we don't control it.
		if c := metav1.GetControllerOf(cd); c != nil && c.UID != cr.GetUID() {
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

// A RenderFn renders the supplied composed resource.
type RenderFn func(cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error

// Render calls RenderFn.
func (c RenderFn) Render(cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	return c(cp, cd, t)
}

// An APIDryRunRenderer renders composed resources. It may perform a dry-run
// create against an API server in order to name and validate the rendered
// resource.
type APIDryRunRenderer struct {
	client client.Client
}

// NewAPIDryRunRenderer returns a Renderer of composed resources that may
// perform a dry-run create against an API server in order to name and validate
// it.
func NewAPIDryRunRenderer(c client.Client) *APIDryRunRenderer {
	return &APIDryRunRenderer{client: c}
}

// Render the supplied composed resource using the supplied composite resource
// and template. The rendered resource may be submitted to an API server via a
// dry run create in order to name and validate it.
func (r *APIDryRunRenderer) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error { //nolint:gocyclo // Only slightly over (11).
	kind := cd.GetObjectKind().GroupVersionKind().Kind
	name := cd.GetName()
	namespace := cd.GetNamespace()

	if err := json.Unmarshal(t.Base.Raw, cd); err != nil {
		return errors.Wrap(err, errUnmarshal)
	}

	// We think this composed resource exists, but when we rendered its template
	// its kind changed. This shouldn't happen. Either someone changed the kind
	// in the template or we're trying to use the wrong template (e.g. because
	// the order of an array of anonymous templates changed).
	if kind != "" && cd.GetObjectKind().GroupVersionKind().Kind != kind {
		return errors.New(errKindChanged)
	}

	if cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] == "" {
		return errors.New(errNamePrefix)
	}

	// Unmarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any. We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] + "-")
	cd.SetName(name)
	cd.SetNamespace(namespace)

	for i := range t.Patches {
		if err := Apply(t.Patches[i], cp, cd, patchTypesFromXR()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
		if env != nil {
			if err := ApplyToObjects(t.Patches[i], env, cd, patchTypesFromToEnvironment()...); err != nil {
				return errors.Wrapf(err, errFmtPatch, i)
			}
		}
	}

	// Composed labels and annotations should be rendered after patches are applied
	meta.AddLabels(cd, map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
		xcrd.LabelKeyClaimName:             cp.GetLabels()[xcrd.LabelKeyClaimName],
		xcrd.LabelKeyClaimNamespace:        cp.GetLabels()[xcrd.LabelKeyClaimNamespace],
	})

	if t.Name != nil {
		SetCompositionResourceName(cd, *t.Name)
	}

	// We do this last to ensure that a Composition cannot influence controller references.
	or := meta.AsController(meta.TypedReferenceTo(cp, cp.GetObjectKind().GroupVersionKind()))
	if err := meta.AddControllerReference(cd, or); err != nil {
		return errors.Wrap(err, errSetControllerRef)
	}

	// We don't want to dry-run create a resource that can't be named by the API
	// server due to a missing generate name. We also don't want to create one
	// that is already named, because doing so will result in an error. The API
	// server seems to respond with a 500 ServerTimeout error for all dry-run
	// failures, so we can't just perform a dry-run and ignore 409 Conflicts for
	// resources that are already named.
	if cd.GetName() != "" || cd.GetGenerateName() == "" {
		return nil
	}

	// The API server returns an available name derived from generateName when
	// we perform a dry-run create. This name is likely (but not guaranteed) to
	// be available when we create the composed resource. If the API server
	// generates a name that is unavailable it will return a 500 ServerTimeout
	// error.
	return errors.Wrap(r.client.Create(ctx, cd, client.DryRunAll), errName)
}

// RenderComposite renders the supplied composite resource using the supplied composed
// resource and template.
func RenderComposite(_ context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, _ *env.Environment) error {
	for i, p := range t.Patches {
		if err := Apply(p, cp, cd, patchTypesToXR()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
	}

	return nil
}

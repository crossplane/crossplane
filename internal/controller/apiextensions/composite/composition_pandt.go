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

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
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
	errGetComposed  = "cannot get composed resource"
	errGCComposed   = "cannot garbage collect composed resource"
	errApply        = "cannot apply composed resource"
	errFetchDetails = "cannot fetch connection details"
	errReadiness    = "cannot check whether composed resource is ready"
	errUnmarshal    = "cannot unmarshal base template"
	errGetSecret    = "cannot get connection secret of composed resource"
	errNamePrefix   = "name prefix is not found in labels"
	errKindChanged  = "cannot change the kind of an existing composed resource"
	errName         = "cannot use dry-run create to name composed resource"
	errInline       = "cannot inline Composition patch sets"
	errRenderCR     = "cannot render composite resource"

	errFmtPatch          = "cannot apply the patch at index %d"
	errFmtConnDetailKey  = "connection detail of type %q key is not set"
	errFmtConnDetailVal  = "connection detail of type %q value is not set"
	errFmtConnDetailPath = "connection detail of type %q fromFieldPath is not set"
	errSetControllerRef  = "cannot set controller reference"
)

// Annotation keys.
const (
	AnnotationKeyCompositionResourceName = "crossplane.io/composition-resource-name"
)

// TODO(negz): Move P&T Composition logic into its own package?

// A PatchAndTransformComposerOption is used to configure a PatchAndTransformComposer.
type PatchAndTransformComposerOption func(*PatchAndTransformComposer)

// WithTemplateAssociator configures how a PatchAndTransformComposer associates
// templates with extant composed resources.
func WithTemplateAssociator(a CompositionTemplateAssociator) PatchAndTransformComposerOption {
	return func(c *PatchAndTransformComposer) {
		c.composition = a
	}
}

// WithCompositeRenderer configures how a PatchAndTransformComposer renders the
// composite resource.
func WithCompositeRenderer(r Renderer) PatchAndTransformComposerOption {
	return func(c *PatchAndTransformComposer) {
		c.composite = r
	}
}

// WithComposedRenderer configures how a PatchAndTransformComposer renders
// composed resources.
func WithComposedRenderer(r Renderer) PatchAndTransformComposerOption {
	return func(c *PatchAndTransformComposer) {
		c.composed.Renderer = r
	}
}

// WithComposedReadinessChecker configures how a PatchAndTransformComposer
// checks composed resource readiness.
func WithComposedReadinessChecker(r ReadinessChecker) PatchAndTransformComposerOption {
	return func(c *PatchAndTransformComposer) {
		c.composed.ReadinessChecker = r
	}
}

// WithComposedConnectionDetailsFetcher configures how a
// PatchAndTransformComposed fetches composed resource connection details.
func WithComposedConnectionDetailsFetcher(f ConnectionDetailsFetcher) PatchAndTransformComposerOption {
	return func(c *PatchAndTransformComposer) {
		c.composed.ConnectionDetailsFetcher = f
	}
}

// A PatchAndTransformComposer composes resources using a Composition's
// 'resources' array, which consist of 'base' resources along with a series of
// patches and transforms.
type PatchAndTransformComposer struct {
	client resource.ClientApplicator

	composite   Renderer
	composition CompositionTemplateAssociator
	composed    composedResource
}

// NewPatchAndTransformComposer returns a Composer that composes resources using
// a Composition's bases, patches, and transforms.
func NewPatchAndTransformComposer(kube client.Client, o ...PatchAndTransformComposerOption) *PatchAndTransformComposer {
	// TODO(negz): Can we avoid double-wrapping if the supplied client is
	// already wrapped? Or just do away with unstructured.NewClient completely?
	kube = unstructured.NewClient(kube)

	c := &PatchAndTransformComposer{
		client: resource.ClientApplicator{Client: kube, Applicator: resource.NewAPIPatchingApplicator(kube)},

		composite:   RendererFn(RenderComposite),
		composition: NewGarbageCollectingAssociator(kube),
		composed: composedResource{
			Renderer:                 NewAPIDryRunRenderer(kube),
			ReadinessChecker:         ReadinessCheckerFn(IsReady),
			ConnectionDetailsFetcher: NewAPIConnectionDetailsFetcher(kube),
		},
	}

	for _, fn := range o {
		fn(c)
	}

	return c
}

// Compose resources using the bases, patches, and transforms specified by the
// supplied Composition.
func (c *PatchAndTransformComposer) Compose(ctx context.Context, cr resource.Composite, comp *v1.Composition, env *env.Environment) (CompositionResult, error) { //nolint:gocyclo // Breaking this up doesn't seem worth yet more layers of abstraction.
	// Inline PatchSets from Composition Spec before composing resources.
	ct, err := ComposedTemplates(comp.Spec)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errInline)
	}

	tas, err := c.composition.AssociateTemplates(ctx, cr, ct)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errAssociate)
	}

	// If we have an environment, run all environment patches before composing
	// resources.
	if env != nil && comp.Spec.Environment != nil {
		for i, p := range comp.Spec.Environment.Patches {
			if err := ApplyEnvironmentPatch(p, cr, env); err != nil {
				return CompositionResult{}, errors.Wrapf(err, errFmtPatchEnvironment, i)
			}
		}
	}

	// We optimistically render all composed resources that we are able to with
	// the expectation that any that we fail to render will subsequently have
	// their error corrected by manual intervention or propagation of a required
	// input. Errors are recorded, but not considered fatal to the composition
	// process.
	refs := make([]corev1.ObjectReference, len(tas))
	cds := make([]pandtState, len(tas))
	for i, ta := range tas {
		cd := composed.New(composed.FromReference(ta.Reference))
		err := c.composed.Render(ctx, cr, cd, ta.Template, env)

		cds[i] = pandtState{
			template:       ta.Template,
			resource:       cd,
			renderError:    err,
			appliedPatches: filterPatches(ta.Template.Patches, patchTypesFromXR()...),
		}
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	// We persist references to our composed resources before we create
	// them. This way we can render composed resources with
	// non-deterministic names, and also potentially recover from any errors
	// we encounter while applying composed resources without leaking them.
	cr.SetResourceReferences(refs)
	if err := c.client.Update(ctx, cr); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	// We apply all of our composed resources before we observe them and
	// update the composite resource accordingly in the loop below. This
	// ensures that issues observing and processing one composed resource
	// won't block the application of another.
	for _, cd := range cds {
		// If we were unable to render the composed resource we should not try
		// and apply it.
		if cd.renderError != nil {
			continue
		}
		if err := c.client.Apply(ctx, cd.resource, append(mergeOptions(cd.appliedPatches), resource.MustBeControllableBy(cr.GetUID()))...); err != nil {
			return CompositionResult{}, errors.Wrap(err, errApply)
		}
	}

	conn := managed.ConnectionDetails{}

	for i := range cds {
		// If we were unable to render the composed resource we should not try
		// to observe it.
		if cds[i].renderError != nil {
			continue
		}

		if err := c.composite.Render(ctx, cr, cds[i].resource, cds[i].template, env); err != nil {
			return CompositionResult{}, errors.Wrap(err, errRenderCR)
		}

		cds[i].conn, err = c.composed.FetchConnectionDetails(ctx, cds[i].resource, cds[i].template)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errFetchDetails)
		}

		for key, val := range cds[i].conn {
			conn[key] = val
		}

		cds[i].ready, err = c.composed.IsReady(ctx, cds[i].resource, cds[i].template)
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
	copy := cr.DeepCopyObject().(client.Object)
	if err := c.client.Apply(ctx, copy, mergeOptions(filterToXRPatches(tas))...); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	out := make([]ComposedResource, len(cds))
	for i := range cds {
		out[i] = cds[i].AsComposedResource()
	}

	return CompositionResult{ConnectionDetails: conn, Composed: out}, nil
}

// pandtState tracks the state of Patch and Transform Composition for a
// particular composed resource.
type pandtState struct {
	template       v1.ComposedTemplate
	resource       resource.Composed
	appliedPatches []v1.Patch
	renderError    error
	conn           managed.ConnectionDetails
	ready          bool
}

func (s pandtState) AsComposedResource() ComposedResource {
	return ComposedResource{
		Name:              pointer.StringDeref(s.template.Name, ""),
		Resource:          s.resource,
		ConnectionDetails: s.conn,
		RenderError:       s.renderError,
		Ready:             s.ready,
	}
}

// SetCompositionResourceName sets the name of the composition template used to
// reconcile a composed resource as an annotation.
func SetCompositionResourceName(o metav1.Object, name string) {
	meta.AddAnnotations(o, map[string]string{AnnotationKeyCompositionResourceName: name})
}

// GetCompositionResourceName gets the name of the composition template used to
// reconcile a composed resource from its annotations.
func GetCompositionResourceName(o metav1.Object) string {
	return o.GetAnnotations()[AnnotationKeyCompositionResourceName]
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

// An APIConnectionDetailsFetcher may use the API server to read connection
// details from a Secret.
type APIConnectionDetailsFetcher struct {
	client client.Client
}

// NewAPIConnectionDetailsFetcher returns a ConnectionDetailsFetcher that may
// use the API server to read connection details from a Secret.
func NewAPIConnectionDetailsFetcher(c client.Client) *APIConnectionDetailsFetcher {
	return &APIConnectionDetailsFetcher{client: c}
}

// FetchConnectionDetails of the supplied composed resource, if any.
func (cdf *APIConnectionDetailsFetcher) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) { //nolint:gocyclo // Relatively simple; complexity is mostly a switch.
	data := map[string][]byte{}
	if sref := cd.GetWriteConnectionSecretToReference(); sref != nil {
		// It's possible that the composed resource does want to write a
		// connection secret but has not yet. We presume this isn't an issue and
		// that we'll propagate any connection details during a future
		// iteration.
		s := &corev1.Secret{}
		nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
		if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
			return nil, errors.Wrap(err, errGetSecret)
		}
		data = s.Data
	}

	conn := managed.ConnectionDetails{}

	for _, d := range t.ConnectionDetails {
		switch tp := connectionDetailType(d); tp {
		case v1.ConnectionDetailTypeFromValue:
			// Name, Value must be set if value type
			switch {
			case d.Name == nil:
				return nil, errors.Errorf(errFmtConnDetailKey, tp)
			case d.Value == nil:
				return nil, errors.Errorf(errFmtConnDetailVal, tp)
			default:
				conn[*d.Name] = []byte(*d.Value)
			}
		case v1.ConnectionDetailTypeFromConnectionSecretKey:
			if d.FromConnectionSecretKey == nil {
				return nil, errors.Errorf(errFmtConnDetailKey, tp)
			}
			if data[*d.FromConnectionSecretKey] == nil {
				// We don't consider this an error because it's possible the
				// key will still be written at some point in the future.
				continue
			}
			key := *d.FromConnectionSecretKey
			if d.Name != nil {
				key = *d.Name
			}
			if key != "" {
				conn[key] = data[*d.FromConnectionSecretKey]
			}
		case v1.ConnectionDetailTypeFromFieldPath:
			switch {
			case d.Name == nil:
				return nil, errors.Errorf(errFmtConnDetailKey, tp)
			case d.FromFieldPath == nil:
				return nil, errors.Errorf(errFmtConnDetailPath, tp)
			default:
				_ = extractFieldPathValue(cd, d, conn)
			}
		case v1.ConnectionDetailTypeUnknown:
			// We weren't able to determine the type of this connection detail.
		}
	}

	if len(conn) == 0 {
		return nil, nil
	}

	return conn, nil
}

// Originally there was no 'type' determinator field so Crossplane would infer
// the type. We maintain this behaviour for backward compatibility when no type
// is set.
func connectionDetailType(d v1.ConnectionDetail) v1.ConnectionDetailType {
	switch {
	case d.Type != nil:
		return *d.Type
	case d.Name != nil && d.Value != nil:
		return v1.ConnectionDetailTypeFromValue
	case d.FromConnectionSecretKey != nil:
		return v1.ConnectionDetailTypeFromConnectionSecretKey
	case d.FromFieldPath != nil:
		return v1.ConnectionDetailTypeFromFieldPath
	default:
		return v1.ConnectionDetailTypeUnknown
	}
}

func extractFieldPathValue(from runtime.Object, detail v1.ConnectionDetail, conn managed.ConnectionDetails) error {
	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	str, err := fieldpath.Pave(fromMap).GetString(*detail.FromFieldPath)
	if err == nil {
		conn[*detail.Name] = []byte(str)
		return nil
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*detail.FromFieldPath)
	if err != nil {
		return err
	}

	buffer, err := json.Marshal(in)
	if err != nil {
		return err
	}
	conn[*detail.Name] = buffer
	return nil
}

// IsReady returns whether the composed resource is ready.
func IsReady(_ context.Context, cd resource.Composed, t v1.ComposedTemplate) (bool, error) { //nolint:gocyclo // Complexity is mostly due to the switch.
	if len(t.ReadinessChecks) == 0 {
		return resource.IsConditionTrue(cd.GetCondition(xpv1.TypeReady)), nil
	}
	// TODO(muvaf): We can probably get rid of resource.Composed interface and fake.Composed
	// structs and use *composed.Unstructured everywhere including tests.
	u, ok := cd.(*composed.Unstructured)
	if !ok {
		return false, errors.New("composed resource has to be Unstructured type")
	}
	paved := fieldpath.Pave(u.UnstructuredContent())

	for i, check := range t.ReadinessChecks {
		var ready bool
		switch check.Type {
		case v1.ReadinessCheckTypeNone:
			return true, nil
		case v1.ReadinessCheckTypeNonEmpty:
			_, err := paved.GetValue(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err)
		case v1.ReadinessCheckTypeMatchString:
			val, err := paved.GetString(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchString
		case v1.ReadinessCheckTypeMatchInteger:
			val, err := paved.GetInteger(check.FieldPath)
			if err != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchInteger
		default:
			return false, errors.Errorf("readiness check at index %d: an unknown type is chosen", i)
		}
		if !ready {
			return false, nil
		}
	}
	return true, nil
}

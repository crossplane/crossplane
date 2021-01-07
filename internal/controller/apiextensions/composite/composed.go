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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings
const (
	errGetComposed     = "cannot get composed resource"
	errGCComposed      = "cannot garbage collect composed resource"
	errApplyComposed   = "cannot apply composed resource"
	errFetchSecret     = "cannot fetch connection secret"
	errReadiness       = "cannot check whether composed resource is ready"
	errUnmarshal       = "cannot unmarshal base template"
	errGetSecret       = "cannot get connection secret of composed resource"
	errKindChanged     = "cannot change the kind of an existing composed resource"
	errNamePrefix      = "name prefix is not found in labels"
	errName            = "cannot use dry-run create to name composed resource"
	errMixed           = "cannot mix named and anonymous resource templates"
	errDuplicate       = "resource template names must be unique within their Composition"
	errRenderComposite = "cannot render composite resource"

	errFmtPatch              = "cannot apply the patch at index %d"
	errFmtRenderComposedIdx  = "cannot render composed resource at index %d"
	errFmtRenderComposedName = "cannot render composed resource template %q"
)

// Annotation keys.
const (
	// TODO(negz): This doesn't belong here.
	AnnotationKeyCompositionTemplateName = "crossplane.io/composition-template-name"
)

// SetCompositionTemplateName sets the name of the composition template used to
// reconcile a composed resource as an annotation.
func SetCompositionTemplateName(o metav1.Object, name string) {
	meta.AddAnnotations(o, map[string]string{AnnotationKeyCompositionTemplateName: name})
}

// GetCompositionTemplateName gets the name of the composition template used to
// reconcile a composed resource from its annotations.
func GetCompositionTemplateName(o metav1.Object) string {
	return o.GetAnnotations()[AnnotationKeyCompositionTemplateName]
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
func (r *APIDryRunRenderer) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
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

	meta.AddLabels(cd, map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
		xcrd.LabelKeyClaimName:             cp.GetLabels()[xcrd.LabelKeyClaimName],
		xcrd.LabelKeyClaimNamespace:        cp.GetLabels()[xcrd.LabelKeyClaimNamespace],
	})

	if t.TemplateName != nil {
		SetCompositionTemplateName(cd, *t.TemplateName)
	}

	// Unmarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any. We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] + "-")
	cd.SetName(name)
	cd.SetNamespace(namespace)
	for i, p := range t.Patches {
		if err := p.Apply(cp, cd); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
	}

	// We do this last to ensure that a Composition cannot influence owner (and
	// especially controller) references.
	or := meta.AsController(meta.TypedReferenceTo(cp, cp.GetObjectKind().GroupVersionKind()))
	cd.SetOwnerReferences([]metav1.OwnerReference{or})

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
func RenderComposite(_ context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	onlyPatches := []v1.PatchType{v1.PatchTypeToCompositeFieldPath}
	for i, p := range t.Patches {
		if err := p.Apply(cp, cd, onlyPatches...); err != nil {
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
func (cdf *APIConnectionDetailsFetcher) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
	sref := cd.GetWriteConnectionSecretToReference()
	if sref == nil {
		return nil, nil
	}

	conn := managed.ConnectionDetails{}

	// It's possible that the composed resource does want to write a
	// connection secret but has not yet. We presume this isn't an issue and
	// that we'll propagate any connection details during a future
	// iteration.
	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
	if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}

	for _, d := range t.ConnectionDetails {
		if d.Name != nil && d.Value != nil {
			conn[*d.Name] = []byte(*d.Value)
			continue
		}

		if d.FromConnectionSecretKey == nil {
			continue
		}

		if len(s.Data[*d.FromConnectionSecretKey]) == 0 {
			continue
		}

		key := *d.FromConnectionSecretKey
		if d.Name != nil {
			key = *d.Name
		}

		conn[key] = s.Data[*d.FromConnectionSecretKey]
	}

	return conn, nil
}

// IsReady returns whether the composed resource is ready.
func IsReady(_ context.Context, cd resource.Composed, t v1.ComposedTemplate) (bool, error) { // nolint:gocyclo
	// NOTE(muvaf): The cyclomatic complexity of this function comes from the
	// mandatory repetitiveness of the switch clause, which is not really complex
	// in reality. Though beware of adding additional complexity besides that.

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
		case v1.ReadinessCheckNone:
			return true, nil
		case v1.ReadinessCheckNonEmpty:
			_, err := paved.GetValue(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err)
		case v1.ReadinessCheckMatchString:
			val, err := paved.GetString(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchString
		case v1.ReadinessCheckMatchInteger:
			val, err := paved.GetInteger(check.FieldPath)
			if err != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchInteger
		default:
			return false, errors.New(fmt.Sprintf("readiness check at index %d: an unknown type is chosen", i))
		}
		if !ready {
			return false, nil
		}
	}
	return true, nil
}

// A Renderer is used to render a composed resource.
type Renderer interface {
	Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error
}

// A RendererFn may be used to render a composed resource.
type RendererFn func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error

// Render the supplied composed resource using the supplied composite resource
// and template as inputs.
func (fn RendererFn) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	return fn(ctx, cp, cd, t)
}

// ConnectionDetailsFetcher fetches the connection details of the Composed resource.
type ConnectionDetailsFetcher interface {
	FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error)
}

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// composed resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error)

// FetchConnectionDetails calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
	return f(ctx, cd, t)
}

// A ReadinessChecker checks whether a composed resource is ready or not.
type ReadinessChecker interface {
	IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)
}

// A ReadinessCheckerFn checks whether a composed resource is ready or not.
type ReadinessCheckerFn func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)

// IsReady reports whether a composed resource is ready or not.
func (fn ReadinessCheckerFn) IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
	return fn(ctx, cd, t)
}

// A CompositionValidator validates the supplied Composition.
type CompositionValidator interface {
	Validate(comp *v1.Composition) error
}

// A CompositionValidatorFn validates the supplied Composition.
type CompositionValidatorFn func(comp *v1.Composition) error

// Validate the supplied Composition.
func (fn CompositionValidatorFn) Validate(comp *v1.Composition) error {
	return fn(comp)
}

// A ValidationChain runs multiple validations.
type ValidationChain []CompositionValidator

// Validate the supplied Composition.
func (vs ValidationChain) Validate(comp *v1.Composition) error {
	for _, v := range vs {
		if err := v.Validate(comp); err != nil {
			return err
		}
	}
	return nil
}

// RejectMixedTemplates validates that the supplied Composition does not attempt
// to mix named and anonymous templates. If some but not all templates are named
// it's safest to refuse to operate. We don't have enough information to use the
// named composer, but using the anonymous composer may be surprising. There's a
// risk that someone added a new anonymous template to a Composition that
// otherwise uses named templates. If they added the new template to the
// beginning or middle of the resources array using the anonymous composer would
// be destructive, because it assumes template N always corresponds to existing
// template N.
func RejectMixedTemplates(comp *v1.Composition) error {
	named := 0
	for _, tmpl := range comp.Spec.Resources {
		if tmpl.TemplateName != nil {
			named++
		}
	}

	// We're using only anonymous templates.
	if named == 0 {
		return nil
	}

	// We're using only named templates.
	if named == len(comp.Spec.Resources) {
		return nil
	}

	return errors.New(errMixed)
}

// RejectDuplicateNames validates that all template names are unique within the
// supplied Composition.
func RejectDuplicateNames(comp *v1.Composition) error {
	seen := map[string]bool{}
	for _, tmpl := range comp.Spec.Resources {
		if tmpl.TemplateName == nil {
			continue
		}
		if seen[*tmpl.TemplateName] {
			return errors.New(errDuplicate)
		}
		seen[*tmpl.TemplateName] = true
	}
	return nil
}

type composedResource struct {
	Renderer
	ConnectionDetailsFetcher
	ReadinessChecker
}

// An AnonymousComposer composes resources using a Composition with anonymous
// (i.e. unnamed) resource templates. It relates composed resources to their
// templates using their index within the composite resource's resource ref
// array and within the composition's resource template array. It assumes the
// length and order of the Composition is immutable; it does not garbage collect
// resources when a template is deleted, and may leak resources when the array
// of templates is reordered.
type AnonymousComposer struct {
	client    resource.ClientApplicator
	composed  composedResource
	composite Renderer
}

// NewAnonymousComposer returns a Composer that composes resources using a
// Composition with anonymous (i.e. unnamed) resource templates.
func NewAnonymousComposer(c client.Client) *AnonymousComposer {
	composed := composedResource{
		Renderer:                 NewAPIDryRunRenderer(c),
		ReadinessChecker:         ReadinessCheckerFn(IsReady),
		ConnectionDetailsFetcher: NewAPIConnectionDetailsFetcher(c),
	}

	return &AnonymousComposer{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		composed:  composed,
		composite: RendererFn(RenderComposite),
	}
}

// Compose resources into the supplied composite per the supplied composition.
func (r *AnonymousComposer) Compose(ctx context.Context, cr resource.Composite, comp *v1.Composition) (CompositionResult, error) { //nolint:gocyclo
	// TODO(negz): Reduce complexity if possible - perhaps extract the common
	// connection details and readiness loop?

	refs := make([]corev1.ObjectReference, len(comp.Spec.Resources))
	copy(refs, cr.GetResourceReferences())

	cds := make([]*composed.Unstructured, len(refs))
	for i := range refs {
		cd := composed.New(composed.FromReference(refs[i]))
		if err := r.composed.Render(ctx, cr, cd, comp.Spec.Resources[i]); err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtRenderComposedIdx, i)
		}

		cds[i] = cd
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	cr.SetResourceReferences(refs)
	if err := r.client.Update(ctx, cr); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	conn := managed.ConnectionDetails{}
	ready := 0
	for i, cd := range cds {
		if err := r.client.Apply(ctx, cd, resource.MustBeControllableBy(cr.GetUID())); err != nil {
			return CompositionResult{}, errors.Wrap(err, errApplyComposed)
		}

		// Connection details are fetched in all cases in a best-effort mode,
		// i.e. it doesn't return error if the secret does not exist or the
		// resource does not publish a secret at all.
		c, err := r.composed.FetchConnectionDetails(ctx, cd, comp.Spec.Resources[i])
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errFetchSecret)
		}

		for key, val := range c {
			conn[key] = val
		}

		rdy, err := r.composed.IsReady(ctx, cd, comp.Spec.Resources[i])
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errReadiness)
		}

		if rdy {
			ready++
		}

		if err := r.composite.Render(ctx, cr, cd, comp.Spec.Resources[i]); err != nil {
			return CompositionResult{}, errors.Wrap(err, errRenderComposite)
		}
	}

	return CompositionResult{ConnectionDetails: conn, DesiredResources: len(comp.Spec.Resources), ReadyResources: ready}, nil
}

// TODO(negz): We may be able to remove the AnonymousComposer if we detected
// Compositions with entirely anonymous templates and automatically populated
// their template names. I'm not sure whether this would reduce the complexity
// of our composition machinery though, given we'd need to use a pass of
// something a lot like the AnonymousComposer logic to propagate our generated
// template names to any existing composed resources.

// A NamedComposer composes resources using a Composition with named templates.
// This allows it to track the relationship between composed resources and the
// templates used to create them, even if templates are reordered or deleted.
type NamedComposer struct {
	client    resource.ClientApplicator
	composed  composedResource
	composite Renderer
	anonymous Composer
}

// NewNamedComposer returns a Composer that composes resources using a
// Composition with named resource templates.
func NewNamedComposer(c client.Client) *NamedComposer {
	composed := composedResource{
		Renderer:                 NewAPIDryRunRenderer(c),
		ReadinessChecker:         ReadinessCheckerFn(IsReady),
		ConnectionDetailsFetcher: NewAPIConnectionDetailsFetcher(c),
	}

	return &NamedComposer{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		composed:  composed,
		composite: RendererFn(RenderComposite),
		anonymous: NewAnonymousComposer(c),
	}
}

// Compose resources into the supplied composite per the supplied composition.
func (r *NamedComposer) Compose(ctx context.Context, cr resource.Composite, comp *v1.Composition) (CompositionResult, error) { //nolint:gocyclo
	// TODO(negz): This logic is uncomfortably complex and should be broken up
	// if we can find a way to do so that doesn't hinder our ability to follow
	// it.

	templates := map[string]v1.ComposedTemplate{}
	for _, t := range comp.Spec.Resources {
		if t.TemplateName != nil {
			templates[*t.TemplateName] = t
		}
	}

	// If no templates are named we should fall back to the anonymous composer.
	if len(templates) == 0 {
		return r.anonymous.Compose(ctx, cr, comp)
	}

	resources := map[string]resource.Composed{}
	for _, ref := range cr.GetResourceReferences() {
		cd := composed.New(composed.FromReference(ref))
		nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if err := r.client.Get(ctx, nn, cd); resource.IgnoreNotFound(err) != nil {
			return CompositionResult{}, errors.Wrap(err, errGetComposed)
		}

		name := GetCompositionTemplateName(cd)
		if meta.WasCreated(cd) && name == "" {
			// All templates are named but this composed resource isn't. We are
			// most likely updating from a Composition with anonymous templates
			// to one with named templates - we need to run the anonymous
			// composer to propagate template names to the existing composed
			// resources.
			return r.anonymous.Compose(ctx, cr, comp)
		}

		resources[name] = cd
	}

	// We ensure all resources can be rendered before we apply any.
	refs := make([]corev1.ObjectReference, len(comp.Spec.Resources))
	for i, tpl := range comp.Spec.Resources {
		cd := composed.New()
		if existing, ok := resources[*tpl.TemplateName]; ok {
			cd.SetNamespace(existing.GetNamespace())
			cd.SetName(existing.GetName())
		}

		if err := r.composed.Render(ctx, cr, cd, tpl); err != nil {
			return CompositionResult{}, errors.Wrapf(err, errFmtRenderComposedName, *tpl.TemplateName)
		}

		resources[*tpl.TemplateName] = cd
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	for name, cd := range resources {
		if _, ok := templates[name]; !ok {
			// This composed resource exists, but the template that was used to
			// create it does not. It should be garbage collected.
			if err := r.client.Delete(ctx, cd); resource.IgnoreNotFound(err) != nil {
				return CompositionResult{}, errors.Wrap(err, errGCComposed)
			}
			continue
		}
	}

	// We need to persist our resource references after garbage collection has
	// succeeded but before we create any new composed resources. If we persist
	// references before garbage collection succeeds we couldn't recover from an
	// error during garbage collection; on the next reconcile we'd have orphaned
	// resources that weren't in our references array. If we don't persist
	// references before we try to create new resources an error during creation
	// may cause the next reconcile to generate new names and thus create
	// duplicate resources.
	cr.SetResourceReferences(refs)
	if err := r.client.Update(ctx, cr); err != nil {
		return CompositionResult{}, errors.Wrap(err, errUpdate)
	}

	// The resources map should now contain resources that were rendered using
	// the current array of templates, as well as any existing resources that no
	// longer have a corresponding template. The former should be applied, while
	// the latter should be deleted.
	conn := managed.ConnectionDetails{}
	ready := 0
	for name, cd := range resources {
		tpl, ok := templates[name]
		if !ok {
			continue
		}

		if err := r.client.Apply(ctx, cd, resource.MustBeControllableBy(cr.GetUID())); err != nil {
			return CompositionResult{}, errors.Wrap(err, errApplyComposed)
		}

		// Connection details are fetched in all cases in a best-effort mode,
		// i.e. we don't return an error if the secret does not exist or the
		// resource does not publish a secret at all.
		c, err := r.composed.FetchConnectionDetails(ctx, cd, tpl)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errFetchSecret)
		}

		for key, val := range c {
			conn[key] = val
		}

		rdy, err := r.composed.IsReady(ctx, cd, tpl)
		if err != nil {
			return CompositionResult{}, errors.Wrap(err, errReadiness)
		}

		if rdy {
			ready++
		}

		if err := r.composite.Render(ctx, cr, cd, tpl); err != nil {
			return CompositionResult{}, errors.Wrap(err, errRenderComposite)
		}
	}

	return CompositionResult{ConnectionDetails: conn, DesiredResources: len(comp.Spec.Resources), ReadyResources: ready}, nil
}

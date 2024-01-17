// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package composite

import (
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings
const (
	errUnmarshalJSON      = "cannot unmarshal JSON data"
	errMarshalProtoStruct = "cannot marshal protobuf Struct to JSON"
	errSetControllerRef   = "cannot set controller reference"

	errFmtKindChanged     = "cannot change the kind of a composed resource from %s to %s (possible composed resource template mismatch)"
	errFmtNamePrefixLabel = "cannot find top-level composite resource name label %q in composite resource metadata"

	// TODO(negz): Include more detail such as field paths if they exist.
	// Perhaps require each patch type to have a String() method to help
	// identify it.
	errFmtPatch = "cannot apply the %q patch at index %d"
)

// RenderFromJSON renders the supplied resource from JSON bytes.
func RenderFromJSON(o resource.Object, data []byte) error {
	gvk := o.GetObjectKind().GroupVersionKind()
	name := o.GetName()
	namespace := o.GetNamespace()

	if err := json.Unmarshal(data, o); err != nil {
		return errors.Wrap(err, errUnmarshalJSON)
	}

	// TODO(negz): Should we return an error if the name or namespace change,
	// rather than just silently re-setting it? Presumably these _changing_ is a
	// sign that something has gone wrong, similar to the GVK changing. What
	// about the UID changing?

	// Unmarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any.
	o.SetName(name)
	o.SetNamespace(namespace)

	// This resource already had a GVK (probably because it already exists), but
	// when we rendered its template it changed. This shouldn't happen. Either
	// someone changed the kind in the template or we're trying to use the wrong
	// template (e.g. because the order of an array of anonymous templates
	// changed).
	if !gvk.Empty() && o.GetObjectKind().GroupVersionKind() != gvk {
		return errors.Errorf(errFmtKindChanged, gvk, o.GetObjectKind().GroupVersionKind())
	}

	return nil
}

// RenderFromCompositeAndEnvironmentPatches renders the supplied composed
// resource by applying all patches that are _from_ the supplied composite
// resource or are from or to the supplied environment.
func RenderFromCompositeAndEnvironmentPatches(cd resource.Composed, xr resource.Composite, e *Environment, p []v1.Patch) error {
	for i := range p {
		if err := Apply(p[i], xr, cd, patchTypesFromXR()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, p[i].Type, i)
		}

		if e != nil {
			if err := ApplyToObjects(p[i], e, cd, patchTypesFromToEnvironment()...); err != nil {
				return errors.Wrapf(err, errFmtPatch, p[i].Type, i)
			}
		}
	}
	return nil
}

// RenderToCompositePatches renders the supplied composite resource by applying
// all patches that are _from_ the supplied composed resource. composed resource
// and template.
func RenderToCompositePatches(xr resource.Composite, cd resource.Composed, p []v1.Patch) error {
	for i := range p {
		if err := Apply(p[i], xr, cd, patchTypesToXR()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, p[i].Type, i)
		}
	}
	return nil
}

// RenderComposedResourceMetadata derives composed resource metadata from the
// supplied composite resource. It makes the composite resource the controller
// of the composed resource. It should run toward the end of a render pipeline
// to ensure that a Composition cannot influence the controller reference.
func RenderComposedResourceMetadata(cd, xr resource.Object, n ResourceName) error {
	// Fail early if the supplied composite resource is missing the name prefix
	// label.
	if xr.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] == "" {
		return errors.Errorf(errFmtNamePrefixLabel, xcrd.LabelKeyNamePrefixForComposed)
	}

	//  We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(xr.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] + "-")

	if n != "" {
		SetCompositionResourceName(cd, n)
	}

	meta.AddLabels(cd, map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: xr.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
		xcrd.LabelKeyClaimName:             xr.GetLabels()[xcrd.LabelKeyClaimName],
		xcrd.LabelKeyClaimNamespace:        xr.GetLabels()[xcrd.LabelKeyClaimNamespace],
	})

	or := meta.AsController(meta.TypedReferenceTo(xr, xr.GetObjectKind().GroupVersionKind()))
	return errors.Wrap(meta.AddControllerReference(cd, or), errSetControllerRef)
}

// TODO(negz): It's simple enough that we should just inline it into the
// PTComposer, which is now the only consumer.

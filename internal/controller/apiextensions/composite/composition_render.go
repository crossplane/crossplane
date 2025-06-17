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
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errUnmarshalJSON      = "cannot unmarshal JSON data"
	errMarshalProtoStruct = "cannot marshal protobuf Struct to JSON"
	errSetControllerRef   = "cannot set controller reference"

	errFmtKindChanged     = "cannot change the kind of a composed resource from %s to %s (possible composed resource template mismatch)"
	errFmtNamePrefixLabel = "cannot find top-level composite resource name label %q in composite resource metadata"
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

	// This resource already had a Kind (probably because it already exists), but
	// when we rendered its template it changed. This shouldn't happen. Either
	// someone changed the kind in the template, or we're trying to use the
	// wrong template (e.g. because the order of an array of anonymous templates
	// changed).
	// Please note, we don't check for version changes, as versions can change. For example,
	// if a composed resource was created with a template that has a version of "v1alpha1",
	// and then the template is updated to "v1beta1", the composed resource will still be valid.
	// We also don't check for group changes, as groups can change during
	// migrations.
	if !gvk.Empty() && o.GetObjectKind().GroupVersionKind().Kind != gvk.Kind {
		return errors.Errorf(errFmtKindChanged, gvk, o.GetObjectKind().GroupVersionKind())
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

	// We recommend composed resources let us generate a name for them. They're
	// allowed to explicitly specify a name if they want though.
	if cd.GetName() == "" && cd.GetGenerateName() == "" {
		cd.SetGenerateName(xr.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] + "-")
	}

	// If the XR is namespaced it can only create composed resources in its own
	// namespace. Cluster scoped XRs can compose cluster scoped resources, or
	// resources in any namespace.
	if xr.GetNamespace() != "" {
		cd.SetNamespace(xr.GetNamespace())
	}

	if n != "" {
		SetCompositionResourceName(cd, n)
	}

	// TODO(negz): What happens if there is no claim? Will this set empty
	// claim name/namespace labels?
	metaLabels := map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: xr.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
	}
	if xr.GetLabels()[xcrd.LabelKeyClaimName] != "" && xr.GetLabels()[xcrd.LabelKeyClaimNamespace] != "" {
		metaLabels[xcrd.LabelKeyClaimName] = xr.GetLabels()[xcrd.LabelKeyClaimName]
		metaLabels[xcrd.LabelKeyClaimNamespace] = xr.GetLabels()[xcrd.LabelKeyClaimNamespace]
	}

	meta.AddLabels(cd, metaLabels)

	or := meta.AsController(meta.TypedReferenceTo(xr, xr.GetObjectKind().GroupVersionKind()))
	return errors.Wrap(meta.AddControllerReference(cd, or), errSetControllerRef)
}

// TODO(negz): It's simple enough that we should just inline it into the
// PTComposer, which is now the only consumer.

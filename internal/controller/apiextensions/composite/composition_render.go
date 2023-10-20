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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings
const (
	errUnmarshalJSON      = "cannot unmarshal JSON data"
	errMarshalProtoStruct = "cannot marshal protobuf Struct to JSON"
	errGenerateName       = "cannot generate a name for a composed resource"
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

// RenderFromCompositePatches renders the supplied composed resource by applying
// all patches that are _from_ the supplied composite resource.
func RenderFromCompositePatches(cd resource.Composed, xr resource.Composite, p []v1.Patch) error {
	for i := range p {
		if err := Apply(p[i], xr, cd, patchTypesFromXR()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, p[i].Type, i)
		}
	}
	return nil
}

// RenderToAndFromEnvironmentPatches renders the supplied composed resource by
// applying all patches that are from or to the supplied environment.
func RenderToAndFromEnvironmentPatches(cd resource.Composed, e *Environment, p []v1.Patch) error {
	if e == nil {
		return nil
	}
	for i := range p {
		if err := ApplyToObjects(p[i], e, cd, patchTypesFromToEnvironment()...); err != nil {
			return errors.Wrapf(err, errFmtPatch, p[i].Type, i)
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

// A NameGenerator finds a name for a composed resource with a
// metadata.generateName value. The name is temporary available, but might be
// taken by the time the composed resource is created.
type NameGenerator interface {
	GenerateName(ctx context.Context, cd resource.Object) error
}

// A NameGeneratorFn is a function that satisfies NameGenerator.
type NameGeneratorFn func(ctx context.Context, cd resource.Object) error

// GenerateName generates a name using the same algorithm as the API server, and
// verifies temporary name availability. It does not submit the composed
// resource to the API server and hence it does not fall over validation errors.
func (fn NameGeneratorFn) GenerateName(ctx context.Context, cd resource.Object) error {
	return fn(ctx, cd)
}

// An APINameGenerator generates a name using the same algorithm as the API
// server and verifies temporary name availability via the API.
type APINameGenerator struct {
	client client.Client
	namer  names.NameGenerator
}

// NewAPINameGenerator returns a new NameGenerator against the API.
func NewAPINameGenerator(c client.Client) *APINameGenerator {
	return &APINameGenerator{client: c, namer: names.SimpleNameGenerator}
}

// GenerateName generates a name using the same algorithm as the API server, and
// verifies temporary name availability. It does not submit the composed
// resource to the API server and hence it does not fall over validation errors.
func (r *APINameGenerator) GenerateName(ctx context.Context, cd resource.Object) error {
	// Don't rename.
	if cd.GetName() != "" || cd.GetGenerateName() == "" {
		return nil
	}

	// We guess a random name and verify that it is available. Names can become
	// unavailable shortly after. Also the client.Get call could be a cache
	// miss. We accepts that very little risk of a name collision though:
	// 1. with 8 million names, a collision against 10k names is 0.01%. We retry
	//    name generation 10 times, to reduce the risks to 0.01%^10, which is
	//    acceptable.
	// 2. the risk that a name gets taken between the client.Get and the
	//    client.Create is that of a name conflict between objects just created
	//    in that short time-span. There are 8 million (minus 10k) free names.
	//    And if there are 100 objects created in parallel, chance of conflict
	//    is 0.06% (birthday paradoxon). This is the best we can do here
	//    locally. To reduce that risk even further the caller must employ a
	//    conflict recovery mechanism.
	maxTries := 10
	for i := 0; i < maxTries; i++ {
		name := r.namer.GenerateName(cd.GetGenerateName())
		obj := composite.Unstructured{}
		obj.SetGroupVersionKind(cd.GetObjectKind().GroupVersionKind())
		err := r.client.Get(ctx, client.ObjectKey{Name: name}, &obj)
		if kerrors.IsNotFound(err) {
			// The name is available.
			cd.SetName(name)
			return nil
		}
		if err != nil {
			return err
		}
	}

	return errors.New(errGenerateName)
}

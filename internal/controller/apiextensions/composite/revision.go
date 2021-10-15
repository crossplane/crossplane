/*
Copyright 2021 The Crossplane Authors.

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
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// AsComposition creates a new composition from the supplied revision. It
// exists only as a temporary translation layer to allow us to introduce the
// alpha CompositionRevision type with minimal changes to the XR reconciler.
// Once CompositionRevision leaves alpha this code should be removed and the XR
// reconciler should operate on CompositionRevisions instead.
func AsComposition(cr *v1alpha1.CompositionRevision) *v1.Composition {
	return &v1.Composition{Spec: AsCompositionSpec(cr.Spec)}
}

// AsCompositionSpec translates a composition revision's spec to a composition
// spec.
func AsCompositionSpec(crs v1alpha1.CompositionRevisionSpec) v1.CompositionSpec {
	cs := v1.CompositionSpec{
		CompositeTypeRef: v1.TypeReference{
			APIVersion: crs.CompositeTypeRef.APIVersion,
			Kind:       crs.CompositeTypeRef.Kind,
		},
		PatchSets:                         make([]v1.PatchSet, len(crs.PatchSets)),
		Resources:                         make([]v1.ComposedTemplate, len(crs.Resources)),
		WriteConnectionSecretsToNamespace: crs.WriteConnectionSecretsToNamespace,
	}

	for i := range crs.PatchSets {
		cs.PatchSets[i] = AsCompositionPatchSet(crs.PatchSets[i])
	}

	for i := range crs.Resources {
		cs.Resources[i] = AsCompositionComposedTemplate(crs.Resources[i])
	}

	return cs
}

// AsCompositionPatchSet translates a composition revision's patch set to a
// composition patch set.
func AsCompositionPatchSet(rps v1alpha1.PatchSet) v1.PatchSet {
	ps := v1.PatchSet{
		Name:    rps.Name,
		Patches: make([]v1.Patch, len(rps.Patches)),
	}

	for i := range rps.Patches {
		ps.Patches[i] = AsCompositionPatch(rps.Patches[i])
	}
	return ps
}

// AsCompositionComposedTemplate translates a composition revision's composed
// (resource) template to a composition composed template.
func AsCompositionComposedTemplate(rct v1alpha1.ComposedTemplate) v1.ComposedTemplate {
	ct := v1.ComposedTemplate{
		Name:              rct.Name,
		Base:              rct.Base,
		Patches:           make([]v1.Patch, len(rct.Patches)),
		ConnectionDetails: make([]v1.ConnectionDetail, len(rct.ConnectionDetails)),
		ReadinessChecks:   make([]v1.ReadinessCheck, len(rct.ReadinessChecks)),
	}

	for i := range rct.Patches {
		ct.Patches[i] = AsCompositionPatch(rct.Patches[i])
	}

	for i := range rct.ConnectionDetails {
		ct.ConnectionDetails[i] = AsCompositionConnectionDetail(rct.ConnectionDetails[i])
	}

	for i := range rct.ReadinessChecks {
		ct.ReadinessChecks[i] = AsCompositionReadinessCheck(rct.ReadinessChecks[i])
	}

	return ct
}

// AsCompositionPatch translates a composition revision's patch to a
// composition patch.
func AsCompositionPatch(rp v1alpha1.Patch) v1.Patch {
	p := v1.Patch{
		Type:          v1.PatchType(rp.Type),
		FromFieldPath: rp.FromFieldPath,
		ToFieldPath:   rp.ToFieldPath,
		PatchSetName:  rp.PatchSetName,
		Transforms:    make([]v1.Transform, len(rp.Transforms)),
	}

	if rp.Combine != nil {
		p.Combine = &v1.Combine{
			Strategy:  v1.CombineStrategy(rp.Combine.Strategy),
			Variables: make([]v1.CombineVariable, len(rp.Combine.Variables)),
		}

		if rp.Combine.String != nil {
			p.Combine.String = &v1.StringCombine{Format: rp.Combine.String.Format}
		}

		for i := range rp.Combine.Variables {
			p.Combine.Variables[i].FromFieldPath = rp.Combine.Variables[i].FromFieldPath
		}
	}

	for i := range rp.Transforms {
		p.Transforms[i] = AsCompositionTransform(rp.Transforms[i])
	}

	if rp.Policy != nil && rp.Policy.FromFieldPath != nil {
		pol := v1.FromFieldPathPolicy(*rp.Policy.FromFieldPath)
		p.Policy = &v1.PatchPolicy{FromFieldPath: &pol}
	}

	return p
}

// AsCompositionTransform translates a compostion revision's transform to a
// composition transform.
func AsCompositionTransform(rt v1alpha1.Transform) v1.Transform {
	t := v1.Transform{Type: v1.TransformType(rt.Type)}
	if rt.Math != nil {
		t.Math = &v1.MathTransform{Multiply: rt.Math.Multiply}
	}
	if rt.Map != nil {
		t.Map = &v1.MapTransform{Pairs: rt.Map.Pairs}
	}
	if rt.String != nil {
		t.String = &v1.StringTransform{Type: v1.StringTransformFormat,
			Format: &rt.String.Format}
	}
	if rt.Convert != nil {
		t.Convert = &v1.ConvertTransform{ToType: rt.Convert.ToType}
	}
	return t
}

// AsCompositionConnectionDetail translates a composition revision's connection
// detail to a composition connection detail.
func AsCompositionConnectionDetail(rcd v1alpha1.ConnectionDetail) v1.ConnectionDetail {
	return v1.ConnectionDetail{
		Name: rcd.Name,
		Type: func() *v1.ConnectionDetailType {
			if rcd.Type == nil {
				return nil
			}
			t := v1.ConnectionDetailType(*rcd.Type)
			return &t
		}(),
		FromConnectionSecretKey: rcd.FromConnectionSecretKey,
		FromFieldPath:           rcd.FromFieldPath,
		Value:                   rcd.Value,
	}
}

// AsCompositionReadinessCheck translates a composition revision's readiness
// check to a composition readiness check.
func AsCompositionReadinessCheck(rrc v1alpha1.ReadinessCheck) v1.ReadinessCheck {
	return v1.ReadinessCheck{
		Type:         v1.ReadinessCheckType(rrc.Type),
		FieldPath:    rrc.FieldPath,
		MatchString:  rrc.MatchString,
		MatchInteger: rrc.MatchInteger,
	}
}

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

package composition

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

// NewCompositionRevision creates a new revision of the supplied Composition.
func NewCompositionRevision(c *v1.Composition, revision int64, compSpecHash string) *v1beta1.CompositionRevision {
	cr := &v1beta1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", c.GetName(), compSpecHash[0:7]),
			Labels: map[string]string{
				v1beta1.LabelCompositionName: c.GetName(),
				// We cannot have a label value longer than 63 chars
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
				v1beta1.LabelCompositionHash: compSpecHash[0:63],
			},
		},
		Spec: NewCompositionRevisionSpec(c.Spec, revision),
	}

	ref := meta.TypedReferenceTo(c, v1.CompositionGroupVersionKind)
	meta.AddOwnerReference(cr, meta.AsController(ref))

	for k, v := range c.GetLabels() {
		cr.ObjectMeta.Labels[k] = v
	}

	return cr
}

// NewCompositionRevisionSpec translates a composition's spec to a composition
// revision spec.
func NewCompositionRevisionSpec(cs v1.CompositionSpec, revision int64) v1beta1.CompositionRevisionSpec {
	rs := v1beta1.CompositionRevisionSpec{
		Revision: revision,
		CompositeTypeRef: v1beta1.TypeReference{
			APIVersion: cs.CompositeTypeRef.APIVersion,
			Kind:       cs.CompositeTypeRef.Kind,
		},
		PatchSets:                         make([]v1beta1.PatchSet, len(cs.PatchSets)),
		Resources:                         make([]v1beta1.ComposedTemplate, len(cs.Resources)),
		WriteConnectionSecretsToNamespace: cs.WriteConnectionSecretsToNamespace,
	}

	if cs.PublishConnectionDetailsWithStoreConfigRef != nil {
		rs.PublishConnectionDetailsWithStoreConfigRef = &v1beta1.StoreConfigReference{Name: cs.PublishConnectionDetailsWithStoreConfigRef.Name}
	}

	for i := range cs.PatchSets {
		rs.PatchSets[i] = NewCompositionRevisionPatchSet(cs.PatchSets[i])
	}

	for i := range cs.Resources {
		rs.Resources[i] = NewCompositionRevisionComposedTemplate(cs.Resources[i])
	}

	return rs
}

// NewCompositionRevisionPatchSet translates a composition's patch set to a
// composition revision patch set.
func NewCompositionRevisionPatchSet(ps v1.PatchSet) v1beta1.PatchSet {
	rps := v1beta1.PatchSet{
		Name:    ps.Name,
		Patches: make([]v1beta1.Patch, len(ps.Patches)),
	}

	for i := range ps.Patches {
		rps.Patches[i] = NewCompositionRevisionPatch(ps.Patches[i])
	}
	return rps
}

// NewCompositionRevisionComposedTemplate translates a composition's composed
// (resource) template to a composition composed template.
func NewCompositionRevisionComposedTemplate(ct v1.ComposedTemplate) v1beta1.ComposedTemplate {
	rct := v1beta1.ComposedTemplate{
		Name:              ct.Name,
		Base:              ct.Base,
		Patches:           make([]v1beta1.Patch, len(ct.Patches)),
		ConnectionDetails: make([]v1beta1.ConnectionDetail, len(ct.ConnectionDetails)),
		ReadinessChecks:   make([]v1beta1.ReadinessCheck, len(ct.ReadinessChecks)),
	}

	for i := range ct.Patches {
		rct.Patches[i] = NewCompositionRevisionPatch(ct.Patches[i])
	}

	for i := range ct.ConnectionDetails {
		rct.ConnectionDetails[i] = NewCompositionRevisionConnectionDetail(ct.ConnectionDetails[i])
	}

	for i := range ct.ReadinessChecks {
		rct.ReadinessChecks[i] = NewCompositionRevisionReadinessCheck(ct.ReadinessChecks[i])
	}

	return rct
}

// NewCompositionRevisionPatch translates a composition's patch to a
// composition revision patch.
func NewCompositionRevisionPatch(p v1.Patch) v1beta1.Patch {
	rp := v1beta1.Patch{
		Type:          v1beta1.PatchType(p.Type),
		FromFieldPath: p.FromFieldPath,
		ToFieldPath:   p.ToFieldPath,
		PatchSetName:  p.PatchSetName,
		Transforms:    make([]v1beta1.Transform, len(p.Transforms)),
	}

	if p.Combine != nil {
		rp.Combine = &v1beta1.Combine{
			Strategy:  v1beta1.CombineStrategy(p.Combine.Strategy),
			Variables: make([]v1beta1.CombineVariable, len(p.Combine.Variables)),
		}

		if p.Combine.String != nil {
			rp.Combine.String = &v1beta1.StringCombine{Format: p.Combine.String.Format}
		}

		for i := range p.Combine.Variables {
			rp.Combine.Variables[i].FromFieldPath = p.Combine.Variables[i].FromFieldPath
		}
	}

	for i := range p.Transforms {
		rp.Transforms[i] = NewCompositionRevisionTransform(p.Transforms[i])
	}

	if p.Policy != nil {
		rp.Policy = &v1beta1.PatchPolicy{}
		if p.Policy.FromFieldPath != nil {
			pol := v1beta1.FromFieldPathPolicy(*p.Policy.FromFieldPath)
			rp.Policy.FromFieldPath = &pol
		}
		if p.Policy.MergeOptions != nil {
			pol := *p.Policy.MergeOptions
			rp.Policy.MergeOptions = &pol
		}
	}

	return rp
}

// NewCompositionRevisionTransform translates a compostion's transform to a
// composition revision transform.
func NewCompositionRevisionTransform(t v1.Transform) v1beta1.Transform { //nolint:gocyclo // Only slightly over (11)
	rt := v1beta1.Transform{Type: v1beta1.TransformType(t.Type)}
	if t.Math != nil {
		rt.Math = &v1beta1.MathTransform{Multiply: t.Math.Multiply}
	}
	if t.Map != nil {
		rt.Map = &v1beta1.MapTransform{Pairs: t.Map.Pairs}
	}
	if t.Match != nil {
		rt.Match = &v1beta1.MatchTransform{
			Patterns:      make([]v1beta1.MatchTransformPattern, len(t.Match.Patterns)),
			FallbackValue: t.Match.FallbackValue,
		}
		for i, p := range t.Match.Patterns {
			rt.Match.Patterns[i] = v1beta1.MatchTransformPattern{
				Type:    v1beta1.MatchTransformPatternType(p.Type),
				Literal: p.Literal,
				Regexp:  p.Regexp,
				Result:  p.Result,
			}
		}
	}
	if t.String != nil {
		rt.String = &v1beta1.StringTransform{Type: v1beta1.StringTransformType(t.String.Type)}
		if t.String.Format != nil {
			rt.String.Format = t.String.Format
		}
		if t.String.Convert != nil {
			rt.String.Convert = func() *v1beta1.StringConversionType {
				t := v1beta1.StringConversionType(*t.String.Convert)
				return &t
			}()
		}
		if t.String.Trim != nil {
			rt.String.Trim = t.String.Trim
		}
		if t.String.Regexp != nil {
			rt.String.Regexp = &v1beta1.StringTransformRegexp{
				Match: t.String.Regexp.Match,
				Group: t.String.Regexp.Group,
			}
		}
	}
	if t.Convert != nil {
		rt.Convert = &v1beta1.ConvertTransform{ToType: t.Convert.ToType}
	}
	return rt
}

// NewCompositionRevisionConnectionDetail translates a composition's connection
// detail to a composition revision connection detail.
func NewCompositionRevisionConnectionDetail(cd v1.ConnectionDetail) v1beta1.ConnectionDetail {
	return v1beta1.ConnectionDetail{
		Name: cd.Name,
		Type: func() *v1beta1.ConnectionDetailType {
			if cd.Type == nil {
				return nil
			}
			t := v1beta1.ConnectionDetailType(*cd.Type)
			return &t
		}(),
		FromConnectionSecretKey: cd.FromConnectionSecretKey,
		FromFieldPath:           cd.FromFieldPath,
		Value:                   cd.Value,
	}
}

// NewCompositionRevisionReadinessCheck translates a composition's readiness
// check to a composition revision readiness check.
func NewCompositionRevisionReadinessCheck(rc v1.ReadinessCheck) v1beta1.ReadinessCheck {
	return v1beta1.ReadinessCheck{
		Type:         v1beta1.ReadinessCheckType(rc.Type),
		FieldPath:    rc.FieldPath,
		MatchString:  rc.MatchString,
		MatchInteger: rc.MatchInteger,
	}
}

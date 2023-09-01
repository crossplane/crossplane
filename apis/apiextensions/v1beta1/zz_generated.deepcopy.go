//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Combine) DeepCopyInto(out *Combine) {
	*out = *in
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]CombineVariable, len(*in))
		copy(*out, *in)
	}
	if in.String != nil {
		in, out := &in.String, &out.String
		*out = new(StringCombine)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Combine.
func (in *Combine) DeepCopy() *Combine {
	if in == nil {
		return nil
	}
	out := new(Combine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CombineVariable) DeepCopyInto(out *CombineVariable) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CombineVariable.
func (in *CombineVariable) DeepCopy() *CombineVariable {
	if in == nil {
		return nil
	}
	out := new(CombineVariable)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComposedTemplate) DeepCopyInto(out *ComposedTemplate) {
	*out = *in
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	in.Base.DeepCopyInto(&out.Base)
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ConnectionDetails != nil {
		in, out := &in.ConnectionDetails, &out.ConnectionDetails
		*out = make([]ConnectionDetail, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ReadinessChecks != nil {
		in, out := &in.ReadinessChecks, &out.ReadinessChecks
		*out = make([]ReadinessCheck, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComposedTemplate.
func (in *ComposedTemplate) DeepCopy() *ComposedTemplate {
	if in == nil {
		return nil
	}
	out := new(ComposedTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CompositionRevision) DeepCopyInto(out *CompositionRevision) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CompositionRevision.
func (in *CompositionRevision) DeepCopy() *CompositionRevision {
	if in == nil {
		return nil
	}
	out := new(CompositionRevision)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CompositionRevision) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CompositionRevisionList) DeepCopyInto(out *CompositionRevisionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CompositionRevision, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CompositionRevisionList.
func (in *CompositionRevisionList) DeepCopy() *CompositionRevisionList {
	if in == nil {
		return nil
	}
	out := new(CompositionRevisionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CompositionRevisionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CompositionRevisionSpec) DeepCopyInto(out *CompositionRevisionSpec) {
	*out = *in
	out.CompositeTypeRef = in.CompositeTypeRef
	if in.Mode != nil {
		in, out := &in.Mode, &out.Mode
		*out = new(CompositionMode)
		**out = **in
	}
	if in.PatchSets != nil {
		in, out := &in.PatchSets, &out.PatchSets
		*out = make([]PatchSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Environment != nil {
		in, out := &in.Environment, &out.Environment
		*out = new(EnvironmentConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ComposedTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Pipeline != nil {
		in, out := &in.Pipeline, &out.Pipeline
		*out = make([]PipelineStep, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.WriteConnectionSecretsToNamespace != nil {
		in, out := &in.WriteConnectionSecretsToNamespace, &out.WriteConnectionSecretsToNamespace
		*out = new(string)
		**out = **in
	}
	if in.PublishConnectionDetailsWithStoreConfigRef != nil {
		in, out := &in.PublishConnectionDetailsWithStoreConfigRef, &out.PublishConnectionDetailsWithStoreConfigRef
		*out = new(StoreConfigReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CompositionRevisionSpec.
func (in *CompositionRevisionSpec) DeepCopy() *CompositionRevisionSpec {
	if in == nil {
		return nil
	}
	out := new(CompositionRevisionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CompositionRevisionStatus) DeepCopyInto(out *CompositionRevisionStatus) {
	*out = *in
	in.ConditionedStatus.DeepCopyInto(&out.ConditionedStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CompositionRevisionStatus.
func (in *CompositionRevisionStatus) DeepCopy() *CompositionRevisionStatus {
	if in == nil {
		return nil
	}
	out := new(CompositionRevisionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConnectionDetail) DeepCopyInto(out *ConnectionDetail) {
	*out = *in
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	if in.Type != nil {
		in, out := &in.Type, &out.Type
		*out = new(ConnectionDetailType)
		**out = **in
	}
	if in.FromConnectionSecretKey != nil {
		in, out := &in.FromConnectionSecretKey, &out.FromConnectionSecretKey
		*out = new(string)
		**out = **in
	}
	if in.FromFieldPath != nil {
		in, out := &in.FromFieldPath, &out.FromFieldPath
		*out = new(string)
		**out = **in
	}
	if in.Value != nil {
		in, out := &in.Value, &out.Value
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConnectionDetail.
func (in *ConnectionDetail) DeepCopy() *ConnectionDetail {
	if in == nil {
		return nil
	}
	out := new(ConnectionDetail)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConvertTransform) DeepCopyInto(out *ConvertTransform) {
	*out = *in
	if in.Format != nil {
		in, out := &in.Format, &out.Format
		*out = new(ConvertTransformFormat)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConvertTransform.
func (in *ConvertTransform) DeepCopy() *ConvertTransform {
	if in == nil {
		return nil
	}
	out := new(ConvertTransform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentConfiguration) DeepCopyInto(out *EnvironmentConfiguration) {
	*out = *in
	if in.DefaultData != nil {
		in, out := &in.DefaultData, &out.DefaultData
		*out = make(map[string]v1.JSON, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.EnvironmentConfigs != nil {
		in, out := &in.EnvironmentConfigs, &out.EnvironmentConfigs
		*out = make([]EnvironmentSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]EnvironmentPatch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Validation != nil {
		in, out := &in.Validation, &out.Validation
		*out = new(EnvironmentValidation)
		(*in).DeepCopyInto(*out)
	}
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(commonv1.Policy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentConfiguration.
func (in *EnvironmentConfiguration) DeepCopy() *EnvironmentConfiguration {
	if in == nil {
		return nil
	}
	out := new(EnvironmentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentPatch) DeepCopyInto(out *EnvironmentPatch) {
	*out = *in
	if in.FromFieldPath != nil {
		in, out := &in.FromFieldPath, &out.FromFieldPath
		*out = new(string)
		**out = **in
	}
	if in.Combine != nil {
		in, out := &in.Combine, &out.Combine
		*out = new(Combine)
		(*in).DeepCopyInto(*out)
	}
	if in.ToFieldPath != nil {
		in, out := &in.ToFieldPath, &out.ToFieldPath
		*out = new(string)
		**out = **in
	}
	if in.Transforms != nil {
		in, out := &in.Transforms, &out.Transforms
		*out = make([]Transform, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(PatchPolicy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentPatch.
func (in *EnvironmentPatch) DeepCopy() *EnvironmentPatch {
	if in == nil {
		return nil
	}
	out := new(EnvironmentPatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentSource) DeepCopyInto(out *EnvironmentSource) {
	*out = *in
	if in.Ref != nil {
		in, out := &in.Ref, &out.Ref
		*out = new(EnvironmentSourceReference)
		**out = **in
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(EnvironmentSourceSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentSource.
func (in *EnvironmentSource) DeepCopy() *EnvironmentSource {
	if in == nil {
		return nil
	}
	out := new(EnvironmentSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentSourceReference) DeepCopyInto(out *EnvironmentSourceReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentSourceReference.
func (in *EnvironmentSourceReference) DeepCopy() *EnvironmentSourceReference {
	if in == nil {
		return nil
	}
	out := new(EnvironmentSourceReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentSourceSelector) DeepCopyInto(out *EnvironmentSourceSelector) {
	*out = *in
	if in.MaxMatch != nil {
		in, out := &in.MaxMatch, &out.MaxMatch
		*out = new(uint64)
		**out = **in
	}
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make([]EnvironmentSourceSelectorLabelMatcher, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentSourceSelector.
func (in *EnvironmentSourceSelector) DeepCopy() *EnvironmentSourceSelector {
	if in == nil {
		return nil
	}
	out := new(EnvironmentSourceSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentSourceSelectorLabelMatcher) DeepCopyInto(out *EnvironmentSourceSelectorLabelMatcher) {
	*out = *in
	if in.ValueFromFieldPath != nil {
		in, out := &in.ValueFromFieldPath, &out.ValueFromFieldPath
		*out = new(string)
		**out = **in
	}
	if in.FromFieldPathPolicy != nil {
		in, out := &in.FromFieldPathPolicy, &out.FromFieldPathPolicy
		*out = new(FromFieldPathPolicy)
		**out = **in
	}
	if in.Value != nil {
		in, out := &in.Value, &out.Value
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentSourceSelectorLabelMatcher.
func (in *EnvironmentSourceSelectorLabelMatcher) DeepCopy() *EnvironmentSourceSelectorLabelMatcher {
	if in == nil {
		return nil
	}
	out := new(EnvironmentSourceSelectorLabelMatcher)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvironmentValidation) DeepCopyInto(out *EnvironmentValidation) {
	*out = *in
	in.JSONSchema.DeepCopyInto(&out.JSONSchema)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvironmentValidation.
func (in *EnvironmentValidation) DeepCopy() *EnvironmentValidation {
	if in == nil {
		return nil
	}
	out := new(EnvironmentValidation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionReference) DeepCopyInto(out *FunctionReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionReference.
func (in *FunctionReference) DeepCopy() *FunctionReference {
	if in == nil {
		return nil
	}
	out := new(FunctionReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MapTransform) DeepCopyInto(out *MapTransform) {
	*out = *in
	if in.Pairs != nil {
		in, out := &in.Pairs, &out.Pairs
		*out = make(map[string]v1.JSON, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MapTransform.
func (in *MapTransform) DeepCopy() *MapTransform {
	if in == nil {
		return nil
	}
	out := new(MapTransform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatchConditionReadinessCheck) DeepCopyInto(out *MatchConditionReadinessCheck) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatchConditionReadinessCheck.
func (in *MatchConditionReadinessCheck) DeepCopy() *MatchConditionReadinessCheck {
	if in == nil {
		return nil
	}
	out := new(MatchConditionReadinessCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatchTransform) DeepCopyInto(out *MatchTransform) {
	*out = *in
	if in.Patterns != nil {
		in, out := &in.Patterns, &out.Patterns
		*out = make([]MatchTransformPattern, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.FallbackValue.DeepCopyInto(&out.FallbackValue)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatchTransform.
func (in *MatchTransform) DeepCopy() *MatchTransform {
	if in == nil {
		return nil
	}
	out := new(MatchTransform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatchTransformPattern) DeepCopyInto(out *MatchTransformPattern) {
	*out = *in
	if in.Literal != nil {
		in, out := &in.Literal, &out.Literal
		*out = new(string)
		**out = **in
	}
	if in.Regexp != nil {
		in, out := &in.Regexp, &out.Regexp
		*out = new(string)
		**out = **in
	}
	in.Result.DeepCopyInto(&out.Result)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatchTransformPattern.
func (in *MatchTransformPattern) DeepCopy() *MatchTransformPattern {
	if in == nil {
		return nil
	}
	out := new(MatchTransformPattern)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MathTransform) DeepCopyInto(out *MathTransform) {
	*out = *in
	if in.Multiply != nil {
		in, out := &in.Multiply, &out.Multiply
		*out = new(int64)
		**out = **in
	}
	if in.ClampMin != nil {
		in, out := &in.ClampMin, &out.ClampMin
		*out = new(int64)
		**out = **in
	}
	if in.ClampMax != nil {
		in, out := &in.ClampMax, &out.ClampMax
		*out = new(int64)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MathTransform.
func (in *MathTransform) DeepCopy() *MathTransform {
	if in == nil {
		return nil
	}
	out := new(MathTransform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Patch) DeepCopyInto(out *Patch) {
	*out = *in
	if in.FromFieldPath != nil {
		in, out := &in.FromFieldPath, &out.FromFieldPath
		*out = new(string)
		**out = **in
	}
	if in.Combine != nil {
		in, out := &in.Combine, &out.Combine
		*out = new(Combine)
		(*in).DeepCopyInto(*out)
	}
	if in.ToFieldPath != nil {
		in, out := &in.ToFieldPath, &out.ToFieldPath
		*out = new(string)
		**out = **in
	}
	if in.PatchSetName != nil {
		in, out := &in.PatchSetName, &out.PatchSetName
		*out = new(string)
		**out = **in
	}
	if in.Transforms != nil {
		in, out := &in.Transforms, &out.Transforms
		*out = make([]Transform, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(PatchPolicy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Patch.
func (in *Patch) DeepCopy() *Patch {
	if in == nil {
		return nil
	}
	out := new(Patch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PatchPolicy) DeepCopyInto(out *PatchPolicy) {
	*out = *in
	if in.FromFieldPath != nil {
		in, out := &in.FromFieldPath, &out.FromFieldPath
		*out = new(FromFieldPathPolicy)
		**out = **in
	}
	if in.MergeOptions != nil {
		in, out := &in.MergeOptions, &out.MergeOptions
		*out = new(commonv1.MergeOptions)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PatchPolicy.
func (in *PatchPolicy) DeepCopy() *PatchPolicy {
	if in == nil {
		return nil
	}
	out := new(PatchPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PatchSet) DeepCopyInto(out *PatchSet) {
	*out = *in
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PatchSet.
func (in *PatchSet) DeepCopy() *PatchSet {
	if in == nil {
		return nil
	}
	out := new(PatchSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineStep) DeepCopyInto(out *PipelineStep) {
	*out = *in
	out.FunctionRef = in.FunctionRef
	if in.Input != nil {
		in, out := &in.Input, &out.Input
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineStep.
func (in *PipelineStep) DeepCopy() *PipelineStep {
	if in == nil {
		return nil
	}
	out := new(PipelineStep)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReadinessCheck) DeepCopyInto(out *ReadinessCheck) {
	*out = *in
	if in.MatchCondition != nil {
		in, out := &in.MatchCondition, &out.MatchCondition
		*out = new(MatchConditionReadinessCheck)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReadinessCheck.
func (in *ReadinessCheck) DeepCopy() *ReadinessCheck {
	if in == nil {
		return nil
	}
	out := new(ReadinessCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StoreConfigReference) DeepCopyInto(out *StoreConfigReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StoreConfigReference.
func (in *StoreConfigReference) DeepCopy() *StoreConfigReference {
	if in == nil {
		return nil
	}
	out := new(StoreConfigReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StringCombine) DeepCopyInto(out *StringCombine) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StringCombine.
func (in *StringCombine) DeepCopy() *StringCombine {
	if in == nil {
		return nil
	}
	out := new(StringCombine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StringTransform) DeepCopyInto(out *StringTransform) {
	*out = *in
	if in.Format != nil {
		in, out := &in.Format, &out.Format
		*out = new(string)
		**out = **in
	}
	if in.Convert != nil {
		in, out := &in.Convert, &out.Convert
		*out = new(StringConversionType)
		**out = **in
	}
	if in.Trim != nil {
		in, out := &in.Trim, &out.Trim
		*out = new(string)
		**out = **in
	}
	if in.Regexp != nil {
		in, out := &in.Regexp, &out.Regexp
		*out = new(StringTransformRegexp)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StringTransform.
func (in *StringTransform) DeepCopy() *StringTransform {
	if in == nil {
		return nil
	}
	out := new(StringTransform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StringTransformRegexp) DeepCopyInto(out *StringTransformRegexp) {
	*out = *in
	if in.Group != nil {
		in, out := &in.Group, &out.Group
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StringTransformRegexp.
func (in *StringTransformRegexp) DeepCopy() *StringTransformRegexp {
	if in == nil {
		return nil
	}
	out := new(StringTransformRegexp)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Transform) DeepCopyInto(out *Transform) {
	*out = *in
	if in.Math != nil {
		in, out := &in.Math, &out.Math
		*out = new(MathTransform)
		(*in).DeepCopyInto(*out)
	}
	if in.Map != nil {
		in, out := &in.Map, &out.Map
		*out = new(MapTransform)
		(*in).DeepCopyInto(*out)
	}
	if in.Match != nil {
		in, out := &in.Match, &out.Match
		*out = new(MatchTransform)
		(*in).DeepCopyInto(*out)
	}
	if in.String != nil {
		in, out := &in.String, &out.String
		*out = new(StringTransform)
		(*in).DeepCopyInto(*out)
	}
	if in.Convert != nil {
		in, out := &in.Convert, &out.Convert
		*out = new(ConvertTransform)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Transform.
func (in *Transform) DeepCopy() *Transform {
	if in == nil {
		return nil
	}
	out := new(Transform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TypeReference) DeepCopyInto(out *TypeReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TypeReference.
func (in *TypeReference) DeepCopy() *TypeReference {
	if in == nil {
		return nil
	}
	out := new(TypeReference)
	in.DeepCopyInto(out)
	return out
}

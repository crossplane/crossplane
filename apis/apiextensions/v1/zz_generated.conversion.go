// Code generated by github.com/jmattheis/goverter, DO NOT EDIT.

package v1

import (
	v13 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v11 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"time"
)

type GeneratedRevisionSpecConverter struct{}

func (c *GeneratedRevisionSpecConverter) FromRevisionSpec(source CompositionRevisionSpec) CompositionSpec {
	var v1CompositionSpec CompositionSpec
	v1CompositionSpec.CompositeTypeRef = c.v1TypeReferenceToV1TypeReference(source.CompositeTypeRef)
	v1PatchSetList := make([]PatchSet, len(source.PatchSets))
	for i := 0; i < len(source.PatchSets); i++ {
		v1PatchSetList[i] = c.v1PatchSetToV1PatchSet(source.PatchSets[i])
	}
	v1CompositionSpec.PatchSets = v1PatchSetList
	var pV1EnvironmentConfiguration *EnvironmentConfiguration
	if source.Environment != nil {
		v1EnvironmentConfiguration := c.v1EnvironmentConfigurationToV1EnvironmentConfiguration(*source.Environment)
		pV1EnvironmentConfiguration = &v1EnvironmentConfiguration
	}
	v1CompositionSpec.Environment = pV1EnvironmentConfiguration
	v1ComposedTemplateList := make([]ComposedTemplate, len(source.Resources))
	for j := 0; j < len(source.Resources); j++ {
		v1ComposedTemplateList[j] = c.v1ComposedTemplateToV1ComposedTemplate(source.Resources[j])
	}
	v1CompositionSpec.Resources = v1ComposedTemplateList
	v1FunctionList := make([]Function, len(source.Functions))
	for k := 0; k < len(source.Functions); k++ {
		v1FunctionList[k] = c.v1FunctionToV1Function(source.Functions[k])
	}
	v1CompositionSpec.Functions = v1FunctionList
	var pString *string
	if source.WriteConnectionSecretsToNamespace != nil {
		xstring := *source.WriteConnectionSecretsToNamespace
		pString = &xstring
	}
	v1CompositionSpec.WriteConnectionSecretsToNamespace = pString
	var pV1StoreConfigReference *StoreConfigReference
	if source.PublishConnectionDetailsWithStoreConfigRef != nil {
		v1StoreConfigReference := c.v1StoreConfigReferenceToV1StoreConfigReference(*source.PublishConnectionDetailsWithStoreConfigRef)
		pV1StoreConfigReference = &v1StoreConfigReference
	}
	v1CompositionSpec.PublishConnectionDetailsWithStoreConfigRef = pV1StoreConfigReference
	return v1CompositionSpec
}
func (c *GeneratedRevisionSpecConverter) ToRevisionSpec(source CompositionSpec) CompositionRevisionSpec {
	var v1CompositionRevisionSpec CompositionRevisionSpec
	v1CompositionRevisionSpec.CompositeTypeRef = c.v1TypeReferenceToV1TypeReference(source.CompositeTypeRef)
	v1PatchSetList := make([]PatchSet, len(source.PatchSets))
	for i := 0; i < len(source.PatchSets); i++ {
		v1PatchSetList[i] = c.v1PatchSetToV1PatchSet(source.PatchSets[i])
	}
	v1CompositionRevisionSpec.PatchSets = v1PatchSetList
	var pV1EnvironmentConfiguration *EnvironmentConfiguration
	if source.Environment != nil {
		v1EnvironmentConfiguration := c.v1EnvironmentConfigurationToV1EnvironmentConfiguration(*source.Environment)
		pV1EnvironmentConfiguration = &v1EnvironmentConfiguration
	}
	v1CompositionRevisionSpec.Environment = pV1EnvironmentConfiguration
	v1ComposedTemplateList := make([]ComposedTemplate, len(source.Resources))
	for j := 0; j < len(source.Resources); j++ {
		v1ComposedTemplateList[j] = c.v1ComposedTemplateToV1ComposedTemplate(source.Resources[j])
	}
	v1CompositionRevisionSpec.Resources = v1ComposedTemplateList
	v1FunctionList := make([]Function, len(source.Functions))
	for k := 0; k < len(source.Functions); k++ {
		v1FunctionList[k] = c.v1FunctionToV1Function(source.Functions[k])
	}
	v1CompositionRevisionSpec.Functions = v1FunctionList
	var pString *string
	if source.WriteConnectionSecretsToNamespace != nil {
		xstring := *source.WriteConnectionSecretsToNamespace
		pString = &xstring
	}
	v1CompositionRevisionSpec.WriteConnectionSecretsToNamespace = pString
	var pV1StoreConfigReference *StoreConfigReference
	if source.PublishConnectionDetailsWithStoreConfigRef != nil {
		v1StoreConfigReference := c.v1StoreConfigReferenceToV1StoreConfigReference(*source.PublishConnectionDetailsWithStoreConfigRef)
		pV1StoreConfigReference = &v1StoreConfigReference
	}
	v1CompositionRevisionSpec.PublishConnectionDetailsWithStoreConfigRef = pV1StoreConfigReference
	return v1CompositionRevisionSpec
}
func (c *GeneratedRevisionSpecConverter) v1CombineToV1Combine(source Combine) Combine {
	var v1Combine Combine
	v1CombineVariableList := make([]CombineVariable, len(source.Variables))
	for i := 0; i < len(source.Variables); i++ {
		v1CombineVariableList[i] = c.v1CombineVariableToV1CombineVariable(source.Variables[i])
	}
	v1Combine.Variables = v1CombineVariableList
	v1Combine.Strategy = CombineStrategy(source.Strategy)
	var pV1StringCombine *StringCombine
	if source.String != nil {
		v1StringCombine := c.v1StringCombineToV1StringCombine(*source.String)
		pV1StringCombine = &v1StringCombine
	}
	v1Combine.String = pV1StringCombine
	return v1Combine
}
func (c *GeneratedRevisionSpecConverter) v1CombineVariableToV1CombineVariable(source CombineVariable) CombineVariable {
	var v1CombineVariable CombineVariable
	v1CombineVariable.FromFieldPath = source.FromFieldPath
	return v1CombineVariable
}
func (c *GeneratedRevisionSpecConverter) v1ComposedTemplateToV1ComposedTemplate(source ComposedTemplate) ComposedTemplate {
	var v1ComposedTemplate ComposedTemplate
	var pString *string
	if source.Name != nil {
		xstring := *source.Name
		pString = &xstring
	}
	v1ComposedTemplate.Name = pString
	v1ComposedTemplate.Base = ConvertRawExtension(source.Base)
	v1PatchList := make([]Patch, len(source.Patches))
	for i := 0; i < len(source.Patches); i++ {
		v1PatchList[i] = c.v1PatchToV1Patch(source.Patches[i])
	}
	v1ComposedTemplate.Patches = v1PatchList
	v1ConnectionDetailList := make([]ConnectionDetail, len(source.ConnectionDetails))
	for j := 0; j < len(source.ConnectionDetails); j++ {
		v1ConnectionDetailList[j] = c.v1ConnectionDetailToV1ConnectionDetail(source.ConnectionDetails[j])
	}
	v1ComposedTemplate.ConnectionDetails = v1ConnectionDetailList
	v1ReadinessCheckList := make([]ReadinessCheck, len(source.ReadinessChecks))
	for k := 0; k < len(source.ReadinessChecks); k++ {
		v1ReadinessCheckList[k] = c.v1ReadinessCheckToV1ReadinessCheck(source.ReadinessChecks[k])
	}
	v1ComposedTemplate.ReadinessChecks = v1ReadinessCheckList
	return v1ComposedTemplate
}
func (c *GeneratedRevisionSpecConverter) v1ConnectionDetailToV1ConnectionDetail(source ConnectionDetail) ConnectionDetail {
	var v1ConnectionDetail ConnectionDetail
	var pString *string
	if source.Name != nil {
		xstring := *source.Name
		pString = &xstring
	}
	v1ConnectionDetail.Name = pString
	var pV1ConnectionDetailType *ConnectionDetailType
	if source.Type != nil {
		v1ConnectionDetailType := ConnectionDetailType(*source.Type)
		pV1ConnectionDetailType = &v1ConnectionDetailType
	}
	v1ConnectionDetail.Type = pV1ConnectionDetailType
	var pString2 *string
	if source.FromConnectionSecretKey != nil {
		xstring2 := *source.FromConnectionSecretKey
		pString2 = &xstring2
	}
	v1ConnectionDetail.FromConnectionSecretKey = pString2
	var pString3 *string
	if source.FromFieldPath != nil {
		xstring3 := *source.FromFieldPath
		pString3 = &xstring3
	}
	v1ConnectionDetail.FromFieldPath = pString3
	var pString4 *string
	if source.Value != nil {
		xstring4 := *source.Value
		pString4 = &xstring4
	}
	v1ConnectionDetail.Value = pString4
	return v1ConnectionDetail
}
func (c *GeneratedRevisionSpecConverter) v1ContainerFunctionNetworkToV1ContainerFunctionNetwork(source ContainerFunctionNetwork) ContainerFunctionNetwork {
	var v1ContainerFunctionNetwork ContainerFunctionNetwork
	var pV1ContainerFunctionNetworkPolicy *ContainerFunctionNetworkPolicy
	if source.Policy != nil {
		v1ContainerFunctionNetworkPolicy := ContainerFunctionNetworkPolicy(*source.Policy)
		pV1ContainerFunctionNetworkPolicy = &v1ContainerFunctionNetworkPolicy
	}
	v1ContainerFunctionNetwork.Policy = pV1ContainerFunctionNetworkPolicy
	return v1ContainerFunctionNetwork
}
func (c *GeneratedRevisionSpecConverter) v1ContainerFunctionResourceLimitsToV1ContainerFunctionResourceLimits(source ContainerFunctionResourceLimits) ContainerFunctionResourceLimits {
	var v1ContainerFunctionResourceLimits ContainerFunctionResourceLimits
	v1ContainerFunctionResourceLimits.CPU = ConvertResourceQuantity(source.CPU)
	v1ContainerFunctionResourceLimits.Memory = ConvertResourceQuantity(source.Memory)
	return v1ContainerFunctionResourceLimits
}
func (c *GeneratedRevisionSpecConverter) v1ContainerFunctionResourcesToV1ContainerFunctionResources(source ContainerFunctionResources) ContainerFunctionResources {
	var v1ContainerFunctionResources ContainerFunctionResources
	var pV1ContainerFunctionResourceLimits *ContainerFunctionResourceLimits
	if source.Limits != nil {
		v1ContainerFunctionResourceLimits := c.v1ContainerFunctionResourceLimitsToV1ContainerFunctionResourceLimits(*source.Limits)
		pV1ContainerFunctionResourceLimits = &v1ContainerFunctionResourceLimits
	}
	v1ContainerFunctionResources.Limits = pV1ContainerFunctionResourceLimits
	return v1ContainerFunctionResources
}
func (c *GeneratedRevisionSpecConverter) v1ContainerFunctionRunnerToV1ContainerFunctionRunner(source ContainerFunctionRunner) ContainerFunctionRunner {
	var v1ContainerFunctionRunner ContainerFunctionRunner
	var pString *string
	if source.Endpoint != nil {
		xstring := *source.Endpoint
		pString = &xstring
	}
	v1ContainerFunctionRunner.Endpoint = pString
	return v1ContainerFunctionRunner
}
func (c *GeneratedRevisionSpecConverter) v1ContainerFunctionToV1ContainerFunction(source ContainerFunction) ContainerFunction {
	var v1ContainerFunction ContainerFunction
	v1ContainerFunction.Image = source.Image
	var pV1PullPolicy *v1.PullPolicy
	if source.ImagePullPolicy != nil {
		v1PullPolicy := v1.PullPolicy(*source.ImagePullPolicy)
		pV1PullPolicy = &v1PullPolicy
	}
	v1ContainerFunction.ImagePullPolicy = pV1PullPolicy
	v1LocalObjectReferenceList := make([]v1.LocalObjectReference, len(source.ImagePullSecrets))
	for i := 0; i < len(source.ImagePullSecrets); i++ {
		v1LocalObjectReferenceList[i] = c.v1LocalObjectReferenceToV1LocalObjectReference(source.ImagePullSecrets[i])
	}
	v1ContainerFunction.ImagePullSecrets = v1LocalObjectReferenceList
	var pV1Duration *v11.Duration
	if source.Timeout != nil {
		v1Duration := c.v1DurationToV1Duration(*source.Timeout)
		pV1Duration = &v1Duration
	}
	v1ContainerFunction.Timeout = pV1Duration
	var pV1ContainerFunctionNetwork *ContainerFunctionNetwork
	if source.Network != nil {
		v1ContainerFunctionNetwork := c.v1ContainerFunctionNetworkToV1ContainerFunctionNetwork(*source.Network)
		pV1ContainerFunctionNetwork = &v1ContainerFunctionNetwork
	}
	v1ContainerFunction.Network = pV1ContainerFunctionNetwork
	var pV1ContainerFunctionResources *ContainerFunctionResources
	if source.Resources != nil {
		v1ContainerFunctionResources := c.v1ContainerFunctionResourcesToV1ContainerFunctionResources(*source.Resources)
		pV1ContainerFunctionResources = &v1ContainerFunctionResources
	}
	v1ContainerFunction.Resources = pV1ContainerFunctionResources
	var pV1ContainerFunctionRunner *ContainerFunctionRunner
	if source.Runner != nil {
		v1ContainerFunctionRunner := c.v1ContainerFunctionRunnerToV1ContainerFunctionRunner(*source.Runner)
		pV1ContainerFunctionRunner = &v1ContainerFunctionRunner
	}
	v1ContainerFunction.Runner = pV1ContainerFunctionRunner
	return v1ContainerFunction
}
func (c *GeneratedRevisionSpecConverter) v1ConvertTransformToV1ConvertTransform(source ConvertTransform) ConvertTransform {
	var v1ConvertTransform ConvertTransform
	v1ConvertTransform.ToType = TransformIOType(source.ToType)
	var pV1ConvertTransformFormat *ConvertTransformFormat
	if source.Format != nil {
		v1ConvertTransformFormat := ConvertTransformFormat(*source.Format)
		pV1ConvertTransformFormat = &v1ConvertTransformFormat
	}
	v1ConvertTransform.Format = pV1ConvertTransformFormat
	return v1ConvertTransform
}
func (c *GeneratedRevisionSpecConverter) v1DurationToV1Duration(source v11.Duration) v11.Duration {
	var v1Duration v11.Duration
	v1Duration.Duration = time.Duration(source.Duration)
	return v1Duration
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentConfigurationToV1EnvironmentConfiguration(source EnvironmentConfiguration) EnvironmentConfiguration {
	var v1EnvironmentConfiguration EnvironmentConfiguration
	v1EnvironmentSourceList := make([]EnvironmentSource, len(source.EnvironmentConfigs))
	for i := 0; i < len(source.EnvironmentConfigs); i++ {
		v1EnvironmentSourceList[i] = c.v1EnvironmentSourceToV1EnvironmentSource(source.EnvironmentConfigs[i])
	}
	v1EnvironmentConfiguration.EnvironmentConfigs = v1EnvironmentSourceList
	v1EnvironmentPatchList := make([]EnvironmentPatch, len(source.Patches))
	for j := 0; j < len(source.Patches); j++ {
		v1EnvironmentPatchList[j] = c.v1EnvironmentPatchToV1EnvironmentPatch(source.Patches[j])
	}
	v1EnvironmentConfiguration.Patches = v1EnvironmentPatchList
	return v1EnvironmentConfiguration
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentPatchToV1EnvironmentPatch(source EnvironmentPatch) EnvironmentPatch {
	var v1EnvironmentPatch EnvironmentPatch
	v1EnvironmentPatch.Type = PatchType(source.Type)
	var pString *string
	if source.FromFieldPath != nil {
		xstring := *source.FromFieldPath
		pString = &xstring
	}
	v1EnvironmentPatch.FromFieldPath = pString
	var pV1Combine *Combine
	if source.Combine != nil {
		v1Combine := c.v1CombineToV1Combine(*source.Combine)
		pV1Combine = &v1Combine
	}
	v1EnvironmentPatch.Combine = pV1Combine
	var pString2 *string
	if source.ToFieldPath != nil {
		xstring2 := *source.ToFieldPath
		pString2 = &xstring2
	}
	v1EnvironmentPatch.ToFieldPath = pString2
	v1TransformList := make([]Transform, len(source.Transforms))
	for i := 0; i < len(source.Transforms); i++ {
		v1TransformList[i] = c.v1TransformToV1Transform(source.Transforms[i])
	}
	v1EnvironmentPatch.Transforms = v1TransformList
	var pV1PatchPolicy *PatchPolicy
	if source.Policy != nil {
		v1PatchPolicy := c.v1PatchPolicyToV1PatchPolicy(*source.Policy)
		pV1PatchPolicy = &v1PatchPolicy
	}
	v1EnvironmentPatch.Policy = pV1PatchPolicy
	return v1EnvironmentPatch
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentSourceReferenceToV1EnvironmentSourceReference(source EnvironmentSourceReference) EnvironmentSourceReference {
	var v1EnvironmentSourceReference EnvironmentSourceReference
	v1EnvironmentSourceReference.Name = source.Name
	return v1EnvironmentSourceReference
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentSourceSelectorLabelMatcherToV1EnvironmentSourceSelectorLabelMatcher(source EnvironmentSourceSelectorLabelMatcher) EnvironmentSourceSelectorLabelMatcher {
	var v1EnvironmentSourceSelectorLabelMatcher EnvironmentSourceSelectorLabelMatcher
	v1EnvironmentSourceSelectorLabelMatcher.Type = EnvironmentSourceSelectorLabelMatcherType(source.Type)
	v1EnvironmentSourceSelectorLabelMatcher.Key = source.Key
	var pString *string
	if source.ValueFromFieldPath != nil {
		xstring := *source.ValueFromFieldPath
		pString = &xstring
	}
	v1EnvironmentSourceSelectorLabelMatcher.ValueFromFieldPath = pString
	var pString2 *string
	if source.Value != nil {
		xstring2 := *source.Value
		pString2 = &xstring2
	}
	v1EnvironmentSourceSelectorLabelMatcher.Value = pString2
	return v1EnvironmentSourceSelectorLabelMatcher
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentSourceSelectorToV1EnvironmentSourceSelector(source EnvironmentSourceSelector) EnvironmentSourceSelector {
	var v1EnvironmentSourceSelector EnvironmentSourceSelector
	v1EnvironmentSourceSelectorLabelMatcherList := make([]EnvironmentSourceSelectorLabelMatcher, len(source.MatchLabels))
	for i := 0; i < len(source.MatchLabels); i++ {
		v1EnvironmentSourceSelectorLabelMatcherList[i] = c.v1EnvironmentSourceSelectorLabelMatcherToV1EnvironmentSourceSelectorLabelMatcher(source.MatchLabels[i])
	}
	v1EnvironmentSourceSelector.MatchLabels = v1EnvironmentSourceSelectorLabelMatcherList
	return v1EnvironmentSourceSelector
}
func (c *GeneratedRevisionSpecConverter) v1EnvironmentSourceToV1EnvironmentSource(source EnvironmentSource) EnvironmentSource {
	var v1EnvironmentSource EnvironmentSource
	v1EnvironmentSource.Type = EnvironmentSourceType(source.Type)
	var pV1EnvironmentSourceReference *EnvironmentSourceReference
	if source.Ref != nil {
		v1EnvironmentSourceReference := c.v1EnvironmentSourceReferenceToV1EnvironmentSourceReference(*source.Ref)
		pV1EnvironmentSourceReference = &v1EnvironmentSourceReference
	}
	v1EnvironmentSource.Ref = pV1EnvironmentSourceReference
	var pV1EnvironmentSourceSelector *EnvironmentSourceSelector
	if source.Selector != nil {
		v1EnvironmentSourceSelector := c.v1EnvironmentSourceSelectorToV1EnvironmentSourceSelector(*source.Selector)
		pV1EnvironmentSourceSelector = &v1EnvironmentSourceSelector
	}
	v1EnvironmentSource.Selector = pV1EnvironmentSourceSelector
	return v1EnvironmentSource
}
func (c *GeneratedRevisionSpecConverter) v1FunctionToV1Function(source Function) Function {
	var v1Function Function
	v1Function.Name = source.Name
	v1Function.Type = FunctionType(source.Type)
	var pRuntimeRawExtension *runtime.RawExtension
	if source.Config != nil {
		runtimeRawExtension := ConvertRawExtension(*source.Config)
		pRuntimeRawExtension = &runtimeRawExtension
	}
	v1Function.Config = pRuntimeRawExtension
	var pV1ContainerFunction *ContainerFunction
	if source.Container != nil {
		v1ContainerFunction := c.v1ContainerFunctionToV1ContainerFunction(*source.Container)
		pV1ContainerFunction = &v1ContainerFunction
	}
	v1Function.Container = pV1ContainerFunction
	return v1Function
}
func (c *GeneratedRevisionSpecConverter) v1JSONToV1JSON(source v12.JSON) v12.JSON {
	var v1JSON v12.JSON
	byteList := make([]uint8, len(source.Raw))
	for i := 0; i < len(source.Raw); i++ {
		byteList[i] = source.Raw[i]
	}
	v1JSON.Raw = byteList
	return v1JSON
}
func (c *GeneratedRevisionSpecConverter) v1LocalObjectReferenceToV1LocalObjectReference(source v1.LocalObjectReference) v1.LocalObjectReference {
	var v1LocalObjectReference v1.LocalObjectReference
	v1LocalObjectReference.Name = source.Name
	return v1LocalObjectReference
}
func (c *GeneratedRevisionSpecConverter) v1MapTransformToV1MapTransform(source MapTransform) MapTransform {
	var v1MapTransform MapTransform
	mapStringV1JSON := make(map[string]v12.JSON, len(source.Pairs))
	for key, value := range source.Pairs {
		mapStringV1JSON[key] = c.v1JSONToV1JSON(value)
	}
	v1MapTransform.Pairs = mapStringV1JSON
	return v1MapTransform
}
func (c *GeneratedRevisionSpecConverter) v1MatchTransformPatternToV1MatchTransformPattern(source MatchTransformPattern) MatchTransformPattern {
	var v1MatchTransformPattern MatchTransformPattern
	v1MatchTransformPattern.Type = MatchTransformPatternType(source.Type)
	var pString *string
	if source.Literal != nil {
		xstring := *source.Literal
		pString = &xstring
	}
	v1MatchTransformPattern.Literal = pString
	var pString2 *string
	if source.Regexp != nil {
		xstring2 := *source.Regexp
		pString2 = &xstring2
	}
	v1MatchTransformPattern.Regexp = pString2
	v1MatchTransformPattern.Result = c.v1JSONToV1JSON(source.Result)
	return v1MatchTransformPattern
}
func (c *GeneratedRevisionSpecConverter) v1MatchTransformToV1MatchTransform(source MatchTransform) MatchTransform {
	var v1MatchTransform MatchTransform
	v1MatchTransformPatternList := make([]MatchTransformPattern, len(source.Patterns))
	for i := 0; i < len(source.Patterns); i++ {
		v1MatchTransformPatternList[i] = c.v1MatchTransformPatternToV1MatchTransformPattern(source.Patterns[i])
	}
	v1MatchTransform.Patterns = v1MatchTransformPatternList
	v1MatchTransform.FallbackValue = c.v1JSONToV1JSON(source.FallbackValue)
	v1MatchTransform.FallbackTo = MatchFallbackTo(source.FallbackTo)
	return v1MatchTransform
}
func (c *GeneratedRevisionSpecConverter) v1MathTransformToV1MathTransform(source MathTransform) MathTransform {
	var v1MathTransform MathTransform
	v1MathTransform.Type = MathTransformType(source.Type)
	var pInt64 *int64
	if source.Multiply != nil {
		xint64 := *source.Multiply
		pInt64 = &xint64
	}
	v1MathTransform.Multiply = pInt64
	var pInt642 *int64
	if source.ClampMin != nil {
		xint642 := *source.ClampMin
		pInt642 = &xint642
	}
	v1MathTransform.ClampMin = pInt642
	var pInt643 *int64
	if source.ClampMax != nil {
		xint643 := *source.ClampMax
		pInt643 = &xint643
	}
	v1MathTransform.ClampMax = pInt643
	return v1MathTransform
}
func (c *GeneratedRevisionSpecConverter) v1MergeOptionsToV1MergeOptions(source v13.MergeOptions) v13.MergeOptions {
	var v1MergeOptions v13.MergeOptions
	var pBool *bool
	if source.KeepMapValues != nil {
		xbool := *source.KeepMapValues
		pBool = &xbool
	}
	v1MergeOptions.KeepMapValues = pBool
	var pBool2 *bool
	if source.AppendSlice != nil {
		xbool2 := *source.AppendSlice
		pBool2 = &xbool2
	}
	v1MergeOptions.AppendSlice = pBool2
	return v1MergeOptions
}
func (c *GeneratedRevisionSpecConverter) v1PatchPolicyToV1PatchPolicy(source PatchPolicy) PatchPolicy {
	var v1PatchPolicy PatchPolicy
	var pV1FromFieldPathPolicy *FromFieldPathPolicy
	if source.FromFieldPath != nil {
		v1FromFieldPathPolicy := FromFieldPathPolicy(*source.FromFieldPath)
		pV1FromFieldPathPolicy = &v1FromFieldPathPolicy
	}
	v1PatchPolicy.FromFieldPath = pV1FromFieldPathPolicy
	var pV1MergeOptions *v13.MergeOptions
	if source.MergeOptions != nil {
		v1MergeOptions := c.v1MergeOptionsToV1MergeOptions(*source.MergeOptions)
		pV1MergeOptions = &v1MergeOptions
	}
	v1PatchPolicy.MergeOptions = pV1MergeOptions
	return v1PatchPolicy
}
func (c *GeneratedRevisionSpecConverter) v1PatchSetToV1PatchSet(source PatchSet) PatchSet {
	var v1PatchSet PatchSet
	v1PatchSet.Name = source.Name
	v1PatchList := make([]Patch, len(source.Patches))
	for i := 0; i < len(source.Patches); i++ {
		v1PatchList[i] = c.v1PatchToV1Patch(source.Patches[i])
	}
	v1PatchSet.Patches = v1PatchList
	return v1PatchSet
}
func (c *GeneratedRevisionSpecConverter) v1PatchToV1Patch(source Patch) Patch {
	var v1Patch Patch
	v1Patch.Type = PatchType(source.Type)
	var pString *string
	if source.FromFieldPath != nil {
		xstring := *source.FromFieldPath
		pString = &xstring
	}
	v1Patch.FromFieldPath = pString
	var pV1Combine *Combine
	if source.Combine != nil {
		v1Combine := c.v1CombineToV1Combine(*source.Combine)
		pV1Combine = &v1Combine
	}
	v1Patch.Combine = pV1Combine
	var pString2 *string
	if source.ToFieldPath != nil {
		xstring2 := *source.ToFieldPath
		pString2 = &xstring2
	}
	v1Patch.ToFieldPath = pString2
	var pString3 *string
	if source.PatchSetName != nil {
		xstring3 := *source.PatchSetName
		pString3 = &xstring3
	}
	v1Patch.PatchSetName = pString3
	v1TransformList := make([]Transform, len(source.Transforms))
	for i := 0; i < len(source.Transforms); i++ {
		v1TransformList[i] = c.v1TransformToV1Transform(source.Transforms[i])
	}
	v1Patch.Transforms = v1TransformList
	var pV1PatchPolicy *PatchPolicy
	if source.Policy != nil {
		v1PatchPolicy := c.v1PatchPolicyToV1PatchPolicy(*source.Policy)
		pV1PatchPolicy = &v1PatchPolicy
	}
	v1Patch.Policy = pV1PatchPolicy
	return v1Patch
}
func (c *GeneratedRevisionSpecConverter) v1ReadinessCheckToV1ReadinessCheck(source ReadinessCheck) ReadinessCheck {
	var v1ReadinessCheck ReadinessCheck
	v1ReadinessCheck.Type = ReadinessCheckType(source.Type)
	v1ReadinessCheck.FieldPath = source.FieldPath
	v1ReadinessCheck.MatchString = source.MatchString
	v1ReadinessCheck.MatchInteger = source.MatchInteger
	return v1ReadinessCheck
}
func (c *GeneratedRevisionSpecConverter) v1StoreConfigReferenceToV1StoreConfigReference(source StoreConfigReference) StoreConfigReference {
	var v1StoreConfigReference StoreConfigReference
	v1StoreConfigReference.Name = source.Name
	return v1StoreConfigReference
}
func (c *GeneratedRevisionSpecConverter) v1StringCombineToV1StringCombine(source StringCombine) StringCombine {
	var v1StringCombine StringCombine
	v1StringCombine.Format = source.Format
	return v1StringCombine
}
func (c *GeneratedRevisionSpecConverter) v1StringTransformRegexpToV1StringTransformRegexp(source StringTransformRegexp) StringTransformRegexp {
	var v1StringTransformRegexp StringTransformRegexp
	v1StringTransformRegexp.Match = source.Match
	var pInt *int
	if source.Group != nil {
		xint := *source.Group
		pInt = &xint
	}
	v1StringTransformRegexp.Group = pInt
	return v1StringTransformRegexp
}
func (c *GeneratedRevisionSpecConverter) v1StringTransformToV1StringTransform(source StringTransform) StringTransform {
	var v1StringTransform StringTransform
	v1StringTransform.Type = StringTransformType(source.Type)
	var pString *string
	if source.Format != nil {
		xstring := *source.Format
		pString = &xstring
	}
	v1StringTransform.Format = pString
	var pV1StringConversionType *StringConversionType
	if source.Convert != nil {
		v1StringConversionType := StringConversionType(*source.Convert)
		pV1StringConversionType = &v1StringConversionType
	}
	v1StringTransform.Convert = pV1StringConversionType
	var pString2 *string
	if source.Trim != nil {
		xstring2 := *source.Trim
		pString2 = &xstring2
	}
	v1StringTransform.Trim = pString2
	var pV1StringTransformRegexp *StringTransformRegexp
	if source.Regexp != nil {
		v1StringTransformRegexp := c.v1StringTransformRegexpToV1StringTransformRegexp(*source.Regexp)
		pV1StringTransformRegexp = &v1StringTransformRegexp
	}
	v1StringTransform.Regexp = pV1StringTransformRegexp
	return v1StringTransform
}
func (c *GeneratedRevisionSpecConverter) v1TransformToV1Transform(source Transform) Transform {
	var v1Transform Transform
	v1Transform.Type = TransformType(source.Type)
	var pV1MathTransform *MathTransform
	if source.Math != nil {
		v1MathTransform := c.v1MathTransformToV1MathTransform(*source.Math)
		pV1MathTransform = &v1MathTransform
	}
	v1Transform.Math = pV1MathTransform
	var pV1MapTransform *MapTransform
	if source.Map != nil {
		v1MapTransform := c.v1MapTransformToV1MapTransform(*source.Map)
		pV1MapTransform = &v1MapTransform
	}
	v1Transform.Map = pV1MapTransform
	var pV1MatchTransform *MatchTransform
	if source.Match != nil {
		v1MatchTransform := c.v1MatchTransformToV1MatchTransform(*source.Match)
		pV1MatchTransform = &v1MatchTransform
	}
	v1Transform.Match = pV1MatchTransform
	var pV1StringTransform *StringTransform
	if source.String != nil {
		v1StringTransform := c.v1StringTransformToV1StringTransform(*source.String)
		pV1StringTransform = &v1StringTransform
	}
	v1Transform.String = pV1StringTransform
	var pV1ConvertTransform *ConvertTransform
	if source.Convert != nil {
		v1ConvertTransform := c.v1ConvertTransformToV1ConvertTransform(*source.Convert)
		pV1ConvertTransform = &v1ConvertTransform
	}
	v1Transform.Convert = pV1ConvertTransform
	return v1Transform
}
func (c *GeneratedRevisionSpecConverter) v1TypeReferenceToV1TypeReference(source TypeReference) TypeReference {
	var v1TypeReference TypeReference
	v1TypeReference.APIVersion = source.APIVersion
	v1TypeReference.Kind = source.Kind
	return v1TypeReference
}

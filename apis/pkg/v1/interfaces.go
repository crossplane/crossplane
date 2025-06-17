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

package v1

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	// LabelParentPackage is used as key for the owner package label we add to the
	// revisions. Its corresponding value should be the name of the owner package.
	LabelParentPackage = "pkg.crossplane.io/package"

	// TODO(negz): Should we propagate the family label up from revision to
	// provider? It could potentially change over time, for example if the
	// active revision's label changed for some reason. There's no technical
	// reason to need it, but being able to list provider.pkg by family seems
	// convenient.

	// LabelProviderFamily is used as key for the provider family label. This
	// label is added to any provider that rolls up to a larger 'family', such
	// as 'family-aws'. It is propagated from provider metadata to provider
	// revisions, and can be used to select all provider revisions that belong
	// to a particular family. It is not added to providers, only revisions.
	LabelProviderFamily = "pkg.crossplane.io/provider-family"
)

var (
	// AutomaticActivation indicates that package should automatically activate
	// package revisions.
	AutomaticActivation RevisionActivationPolicy = "Automatic"
	// ManualActivation indicates that a user will manually activate package
	// revisions.
	ManualActivation RevisionActivationPolicy = "Manual"
)

// RefNames converts a slice of LocalObjectReferences to a slice of strings.
func RefNames(refs []corev1.LocalObjectReference) []string {
	stringRefs := make([]string, len(refs))
	for i, ref := range refs {
		stringRefs[i] = ref.Name
	}
	return stringRefs
}

// TODO(negz): Move these interfaces out of the apis package and closer to where
// they're consumed. This'll probably require internal duplicates of some of the
// returned types. We could generate converters from the public API types to the
// internal types, like we do for the Usage types.

// PackageWithRuntime is the interface satisfied by packages with runtime types.
// +k8s:deepcopy-gen=false
type PackageWithRuntime interface {
	Package

	GetRuntimeConfigRef() *RuntimeConfigReference
	SetRuntimeConfigRef(r *RuntimeConfigReference)

	GetTLSServerSecretName() *string

	GetTLSClientSecretName() *string
}

// SetAppliedImageConfigRefs sets applied image config refs, replacing any
// existing refs with the same reason.
func (s *PackageStatus) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	for _, ref := range refs {
		exists := false
		for i, existing := range s.AppliedImageConfigRefs {
			if existing.Reason != ref.Reason {
				continue
			}
			s.AppliedImageConfigRefs[i] = ref
			exists = true
		}
		if !exists {
			s.AppliedImageConfigRefs = append(s.AppliedImageConfigRefs, ref)
		}
	}
}

// ClearAppliedImageConfigRef removes the applied image config ref with the
// given reason.
func (s *PackageStatus) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	for i, ref := range s.AppliedImageConfigRefs {
		if ref.Reason == reason {
			// There should only be one ref with the given reason; remove it and
			// return.
			s.AppliedImageConfigRefs = slices.Delete(s.AppliedImageConfigRefs, i, i+1)
			break
		}
	}
}

// Package is the interface satisfied by package types.
// +k8s:deepcopy-gen=false
type Package interface { //nolint:interfacebloat // TODO(negz): Could we break this up into smaller, composable interfaces?
	resource.Object
	resource.Conditioned

	CleanConditions()

	GetSource() string
	SetSource(s string)

	GetActivationPolicy() *RevisionActivationPolicy
	SetActivationPolicy(a *RevisionActivationPolicy)

	GetPackagePullSecrets() []corev1.LocalObjectReference
	SetPackagePullSecrets(s []corev1.LocalObjectReference)

	GetPackagePullPolicy() *corev1.PullPolicy
	SetPackagePullPolicy(i *corev1.PullPolicy)

	GetRevisionHistoryLimit() *int64
	SetRevisionHistoryLimit(l *int64)

	GetIgnoreCrossplaneConstraints() *bool
	SetIgnoreCrossplaneConstraints(b *bool)

	GetCurrentRevision() string
	SetCurrentRevision(r string)

	GetCurrentIdentifier() string
	SetCurrentIdentifier(r string)

	GetSkipDependencyResolution() *bool
	SetSkipDependencyResolution(skip *bool)

	GetCommonLabels() map[string]string
	SetCommonLabels(l map[string]string)

	GetAppliedImageConfigRefs() []ImageConfigRef
	SetAppliedImageConfigRefs(refs ...ImageConfigRef)
	ClearAppliedImageConfigRef(reason ImageConfigRefReason)

	GetResolvedSource() string
	SetResolvedSource(s string)
}

// GetCondition of this Provider.
func (p *Provider) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Provider.
func (p *Provider) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (p *Provider) CleanConditions() {
	p.Status.Conditions = []xpv1.Condition{}
}

// GetSource of this Provider.
func (p *Provider) GetSource() string {
	return p.Spec.Package
}

// SetSource of this Provider.
func (p *Provider) SetSource(s string) {
	p.Spec.Package = s
}

// GetActivationPolicy of this Provider.
func (p *Provider) GetActivationPolicy() *RevisionActivationPolicy {
	return p.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Provider.
func (p *Provider) SetActivationPolicy(a *RevisionActivationPolicy) {
	p.Spec.RevisionActivationPolicy = a
}

// GetPackagePullSecrets of this Provider.
func (p *Provider) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return p.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this Provider.
func (p *Provider) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	p.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this Provider.
func (p *Provider) GetPackagePullPolicy() *corev1.PullPolicy {
	return p.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this Provider.
func (p *Provider) SetPackagePullPolicy(i *corev1.PullPolicy) {
	p.Spec.PackagePullPolicy = i
}

// GetRevisionHistoryLimit of this Provider.
func (p *Provider) GetRevisionHistoryLimit() *int64 {
	return p.Spec.RevisionHistoryLimit
}

// SetRevisionHistoryLimit of this Provider.
func (p *Provider) SetRevisionHistoryLimit(l *int64) {
	p.Spec.RevisionHistoryLimit = l
}

// GetIgnoreCrossplaneConstraints of this Provider.
func (p *Provider) GetIgnoreCrossplaneConstraints() *bool {
	return p.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this Provider.
func (p *Provider) SetIgnoreCrossplaneConstraints(b *bool) {
	p.Spec.IgnoreCrossplaneConstraints = b
}

// GetRuntimeConfigRef of this Provider.
func (p *Provider) GetRuntimeConfigRef() *RuntimeConfigReference {
	return p.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this Provider.
func (p *Provider) SetRuntimeConfigRef(r *RuntimeConfigReference) {
	p.Spec.RuntimeConfigReference = r
}

// GetCurrentRevision of this Provider.
func (p *Provider) GetCurrentRevision() string {
	return p.Status.CurrentRevision
}

// SetCurrentRevision of this Provider.
func (p *Provider) SetCurrentRevision(s string) {
	p.Status.CurrentRevision = s
}

// GetSkipDependencyResolution of this Provider.
func (p *Provider) GetSkipDependencyResolution() *bool {
	return p.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this Provider.
func (p *Provider) SetSkipDependencyResolution(b *bool) {
	p.Spec.SkipDependencyResolution = b
}

// GetCurrentIdentifier of this Provider.
func (p *Provider) GetCurrentIdentifier() string {
	return p.Status.CurrentIdentifier
}

// SetCurrentIdentifier of this Provider.
func (p *Provider) SetCurrentIdentifier(s string) {
	p.Status.CurrentIdentifier = s
}

// GetCommonLabels of this Provider.
func (p *Provider) GetCommonLabels() map[string]string {
	return p.Spec.CommonLabels
}

// SetCommonLabels of this Provider.
func (p *Provider) SetCommonLabels(l map[string]string) {
	p.Spec.CommonLabels = l
}

// GetTLSServerSecretName of this Provider.
func (p *Provider) GetTLSServerSecretName() *string {
	return GetSecretNameWithSuffix(p.GetName(), TLSServerSecretNameSuffix)
}

// GetTLSClientSecretName of this Provider.
func (p *Provider) GetTLSClientSecretName() *string {
	return GetSecretNameWithSuffix(p.GetName(), TLSClientSecretNameSuffix)
}

// GetAppliedImageConfigRefs of this Provider.
func (p *Provider) GetAppliedImageConfigRefs() []ImageConfigRef {
	return p.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this Provider.
func (p *Provider) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	p.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this Provider.
func (p *Provider) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	p.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this Provider.
func (p *Provider) GetResolvedSource() string {
	return p.Status.ResolvedPackage
}

// SetResolvedSource of this Provider.
func (p *Provider) SetResolvedSource(s string) {
	p.Status.ResolvedPackage = s
}

// GetCondition of this Configuration.
func (p *Configuration) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Configuration.
func (p *Configuration) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (p *Configuration) CleanConditions() {
	p.Status.Conditions = []xpv1.Condition{}
}

// GetSource of this Configuration.
func (p *Configuration) GetSource() string {
	return p.Spec.Package
}

// SetSource of this Configuration.
func (p *Configuration) SetSource(s string) {
	p.Spec.Package = s
}

// GetActivationPolicy of this Configuration.
func (p *Configuration) GetActivationPolicy() *RevisionActivationPolicy {
	return p.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Configuration.
func (p *Configuration) SetActivationPolicy(a *RevisionActivationPolicy) {
	p.Spec.RevisionActivationPolicy = a
}

// GetPackagePullSecrets of this Configuration.
func (p *Configuration) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return p.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this Configuration.
func (p *Configuration) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	p.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this Configuration.
func (p *Configuration) GetPackagePullPolicy() *corev1.PullPolicy {
	return p.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this Configuration.
func (p *Configuration) SetPackagePullPolicy(i *corev1.PullPolicy) {
	p.Spec.PackagePullPolicy = i
}

// GetRevisionHistoryLimit of this Configuration.
func (p *Configuration) GetRevisionHistoryLimit() *int64 {
	return p.Spec.RevisionHistoryLimit
}

// SetRevisionHistoryLimit of this Configuration.
func (p *Configuration) SetRevisionHistoryLimit(l *int64) {
	p.Spec.RevisionHistoryLimit = l
}

// GetIgnoreCrossplaneConstraints of this Configuration.
func (p *Configuration) GetIgnoreCrossplaneConstraints() *bool {
	return p.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this Configuration.
func (p *Configuration) SetIgnoreCrossplaneConstraints(b *bool) {
	p.Spec.IgnoreCrossplaneConstraints = b
}

// GetCurrentRevision of this Configuration.
func (p *Configuration) GetCurrentRevision() string {
	return p.Status.CurrentRevision
}

// SetCurrentRevision of this Configuration.
func (p *Configuration) SetCurrentRevision(s string) {
	p.Status.CurrentRevision = s
}

// GetSkipDependencyResolution of this Configuration.
func (p *Configuration) GetSkipDependencyResolution() *bool {
	return p.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this Configuration.
func (p *Configuration) SetSkipDependencyResolution(b *bool) {
	p.Spec.SkipDependencyResolution = b
}

// GetCurrentIdentifier of this Configuration.
func (p *Configuration) GetCurrentIdentifier() string {
	return p.Status.CurrentIdentifier
}

// SetCurrentIdentifier of this Configuration.
func (p *Configuration) SetCurrentIdentifier(s string) {
	p.Status.CurrentIdentifier = s
}

// GetCommonLabels of this Configuration.
func (p *Configuration) GetCommonLabels() map[string]string {
	return p.Spec.CommonLabels
}

// SetCommonLabels of this Configuration.
func (p *Configuration) SetCommonLabels(l map[string]string) {
	p.Spec.CommonLabels = l
}

// GetAppliedImageConfigRefs of this Configuration.
func (p *Configuration) GetAppliedImageConfigRefs() []ImageConfigRef {
	return p.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this Configuration.
func (p *Configuration) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	p.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this Configuration.
func (p *Configuration) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	p.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this Configuration.
func (p *Configuration) GetResolvedSource() string {
	return p.Status.ResolvedPackage
}

// SetResolvedSource of this Configuration.
func (p *Configuration) SetResolvedSource(s string) {
	p.Status.ResolvedPackage = s
}

// PackageRevisionWithRuntime is the interface satisfied by revision of packages
// with runtime types.
// +k8s:deepcopy-gen=false
type PackageRevisionWithRuntime interface { //nolint:interfacebloat // TODO(negz): Could this be composed of smaller interfaces?
	PackageRevision

	GetRuntimeConfigRef() *RuntimeConfigReference
	SetRuntimeConfigRef(r *RuntimeConfigReference)

	GetTLSServerSecretName() *string
	SetTLSServerSecretName(n *string)

	GetTLSClientSecretName() *string
	SetTLSClientSecretName(n *string)
}

// SetAppliedImageConfigRefs sets applied image config refs, replacing any
// existing refs with the same reason.
func (s *PackageRevisionStatus) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	for _, ref := range refs {
		exists := false
		for i, existing := range s.AppliedImageConfigRefs {
			if existing.Reason != ref.Reason {
				continue
			}
			s.AppliedImageConfigRefs[i] = ref
			exists = true
		}
		if !exists {
			s.AppliedImageConfigRefs = append(s.AppliedImageConfigRefs, ref)
		}
	}
}

// ClearAppliedImageConfigRef removes the applied image config ref with the
// given reason.
func (s *PackageRevisionStatus) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	for i, ref := range s.AppliedImageConfigRefs {
		if ref.Reason == reason {
			// There should only be one ref with the given reason; remove it and
			// return.
			s.AppliedImageConfigRefs = slices.Delete(s.AppliedImageConfigRefs, i, i+1)
			break
		}
	}
}

// PackageRevision is the interface satisfied by package revision types.
// +k8s:deepcopy-gen=false
type PackageRevision interface { //nolint:interfacebloat // TODO(negz): Could we break this up into smaller, composable interfaces?
	resource.Object
	resource.Conditioned

	CleanConditions()

	GetObjects() []xpv1.TypedReference
	SetObjects(c []xpv1.TypedReference)

	GetSource() string
	SetSource(s string)

	GetPackagePullSecrets() []corev1.LocalObjectReference
	SetPackagePullSecrets(s []corev1.LocalObjectReference)

	GetPackagePullPolicy() *corev1.PullPolicy
	SetPackagePullPolicy(i *corev1.PullPolicy)

	GetDesiredState() PackageRevisionDesiredState
	SetDesiredState(d PackageRevisionDesiredState)

	GetIgnoreCrossplaneConstraints() *bool
	SetIgnoreCrossplaneConstraints(b *bool)

	GetRevision() int64
	SetRevision(r int64)

	GetSkipDependencyResolution() *bool
	SetSkipDependencyResolution(skip *bool)

	GetDependencyStatus() (found, installed, invalid int64)
	SetDependencyStatus(found, installed, invalid int64)

	GetCommonLabels() map[string]string
	SetCommonLabels(l map[string]string)

	GetAppliedImageConfigRefs() []ImageConfigRef
	SetAppliedImageConfigRefs(refs ...ImageConfigRef)
	ClearAppliedImageConfigRef(reason ImageConfigRefReason)

	GetResolvedSource() string
	SetResolvedSource(s string)
}

// GetCondition of this ProviderRevision.
func (p *ProviderRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ProviderRevision.
func (p *ProviderRevision) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (p *ProviderRevision) CleanConditions() {
	p.Status.Conditions = []xpv1.Condition{}
}

// GetObjects of this ProviderRevision.
func (p *ProviderRevision) GetObjects() []xpv1.TypedReference {
	return p.Status.ObjectRefs
}

// SetObjects of this ProviderRevision.
func (p *ProviderRevision) SetObjects(c []xpv1.TypedReference) {
	p.Status.ObjectRefs = c
}

// GetSource of this ProviderRevision.
func (p *ProviderRevision) GetSource() string {
	return p.Spec.Package
}

// SetSource of this ProviderRevision.
func (p *ProviderRevision) SetSource(s string) {
	p.Spec.Package = s
}

// GetPackagePullSecrets of this ProviderRevision.
func (p *ProviderRevision) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return p.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this ProviderRevision.
func (p *ProviderRevision) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	p.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this ProviderRevision.
func (p *ProviderRevision) GetPackagePullPolicy() *corev1.PullPolicy {
	return p.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this ProviderRevision.
func (p *ProviderRevision) SetPackagePullPolicy(i *corev1.PullPolicy) {
	p.Spec.PackagePullPolicy = i
}

// GetDesiredState of this ProviderRevision.
func (p *ProviderRevision) GetDesiredState() PackageRevisionDesiredState {
	return p.Spec.DesiredState
}

// SetDesiredState of this ProviderRevision.
func (p *ProviderRevision) SetDesiredState(s PackageRevisionDesiredState) {
	p.Spec.DesiredState = s
}

// GetRevision of this ProviderRevision.
func (p *ProviderRevision) GetRevision() int64 {
	return p.Spec.Revision
}

// SetRevision of this ProviderRevision.
func (p *ProviderRevision) SetRevision(r int64) {
	p.Spec.Revision = r
}

// GetDependencyStatus of this ProviderRevision.
func (p *ProviderRevision) GetDependencyStatus() (found, installed, invalid int64) {
	return p.Status.FoundDependencies, p.Status.InstalledDependencies, p.Status.InvalidDependencies
}

// SetDependencyStatus of this ProviderRevision.
func (p *ProviderRevision) SetDependencyStatus(found, installed, invalid int64) {
	p.Status.FoundDependencies = found
	p.Status.InstalledDependencies = installed
	p.Status.InvalidDependencies = invalid
}

// GetIgnoreCrossplaneConstraints of this ProviderRevision.
func (p *ProviderRevision) GetIgnoreCrossplaneConstraints() *bool {
	return p.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this ProviderRevision.
func (p *ProviderRevision) SetIgnoreCrossplaneConstraints(b *bool) {
	p.Spec.IgnoreCrossplaneConstraints = b
}

// GetRuntimeConfigRef of this ProviderRevision.
func (p *ProviderRevision) GetRuntimeConfigRef() *RuntimeConfigReference {
	return p.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this ProviderRevision.
func (p *ProviderRevision) SetRuntimeConfigRef(r *RuntimeConfigReference) {
	p.Spec.RuntimeConfigReference = r
}

// GetSkipDependencyResolution of this ProviderRevision.
func (p *ProviderRevision) GetSkipDependencyResolution() *bool {
	return p.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this ProviderRevision.
func (p *ProviderRevision) SetSkipDependencyResolution(b *bool) {
	p.Spec.SkipDependencyResolution = b
}

// GetTLSServerSecretName of this ProviderRevision.
func (p *ProviderRevision) GetTLSServerSecretName() *string {
	return p.Spec.TLSServerSecretName
}

// SetTLSServerSecretName of this ProviderRevision.
func (p *ProviderRevision) SetTLSServerSecretName(s *string) {
	p.Spec.TLSServerSecretName = s
}

// GetTLSClientSecretName of this ProviderRevision.
func (p *ProviderRevision) GetTLSClientSecretName() *string {
	return p.Spec.TLSClientSecretName
}

// SetTLSClientSecretName of this ProviderRevision.
func (p *ProviderRevision) SetTLSClientSecretName(s *string) {
	p.Spec.TLSClientSecretName = s
}

// GetCommonLabels of this ProviderRevision.
func (p *ProviderRevision) GetCommonLabels() map[string]string {
	return p.Spec.CommonLabels
}

// SetCommonLabels of this ProviderRevision.
func (p *ProviderRevision) SetCommonLabels(l map[string]string) {
	p.Spec.CommonLabels = l
}

// GetAppliedImageConfigRefs of this ProviderRevision.
func (p *ProviderRevision) GetAppliedImageConfigRefs() []ImageConfigRef {
	return p.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this ProviderRevision.
func (p *ProviderRevision) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	p.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this ProviderRevision.
func (p *ProviderRevision) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	p.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this ProviderRevision.
func (p *ProviderRevision) GetResolvedSource() string {
	return p.Status.ResolvedPackage
}

// SetResolvedSource of this ProviderRevision.
func (p *ProviderRevision) SetResolvedSource(s string) {
	p.Status.ResolvedPackage = s
}

// GetCondition of this ConfigurationRevision.
func (p *ConfigurationRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ConfigurationRevision.
func (p *ConfigurationRevision) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (p *ConfigurationRevision) CleanConditions() {
	p.Status.Conditions = []xpv1.Condition{}
}

// GetObjects of this ConfigurationRevision.
func (p *ConfigurationRevision) GetObjects() []xpv1.TypedReference {
	return p.Status.ObjectRefs
}

// SetObjects of this ConfigurationRevision.
func (p *ConfigurationRevision) SetObjects(c []xpv1.TypedReference) {
	p.Status.ObjectRefs = c
}

// GetSource of this ConfigurationRevision.
func (p *ConfigurationRevision) GetSource() string {
	return p.Spec.Package
}

// SetSource of this ConfigurationRevision.
func (p *ConfigurationRevision) SetSource(s string) {
	p.Spec.Package = s
}

// GetPackagePullSecrets of this ConfigurationRevision.
func (p *ConfigurationRevision) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return p.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this ConfigurationRevision.
func (p *ConfigurationRevision) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	p.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this ConfigurationRevision.
func (p *ConfigurationRevision) GetPackagePullPolicy() *corev1.PullPolicy {
	return p.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this ConfigurationRevision.
func (p *ConfigurationRevision) SetPackagePullPolicy(i *corev1.PullPolicy) {
	p.Spec.PackagePullPolicy = i
}

// GetDesiredState of this ConfigurationRevision.
func (p *ConfigurationRevision) GetDesiredState() PackageRevisionDesiredState {
	return p.Spec.DesiredState
}

// SetDesiredState of this ConfigurationRevision.
func (p *ConfigurationRevision) SetDesiredState(s PackageRevisionDesiredState) {
	p.Spec.DesiredState = s
}

// GetRevision of this ConfigurationRevision.
func (p *ConfigurationRevision) GetRevision() int64 {
	return p.Spec.Revision
}

// SetRevision of this ConfigurationRevision.
func (p *ConfigurationRevision) SetRevision(r int64) {
	p.Spec.Revision = r
}

// GetDependencyStatus of this v.
func (p *ConfigurationRevision) GetDependencyStatus() (found, installed, invalid int64) {
	return p.Status.FoundDependencies, p.Status.InstalledDependencies, p.Status.InvalidDependencies
}

// SetDependencyStatus of this ConfigurationRevision.
func (p *ConfigurationRevision) SetDependencyStatus(found, installed, invalid int64) {
	p.Status.FoundDependencies = found
	p.Status.InstalledDependencies = installed
	p.Status.InvalidDependencies = invalid
}

// GetIgnoreCrossplaneConstraints of this ConfigurationRevision.
func (p *ConfigurationRevision) GetIgnoreCrossplaneConstraints() *bool {
	return p.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this ConfigurationRevision.
func (p *ConfigurationRevision) SetIgnoreCrossplaneConstraints(b *bool) {
	p.Spec.IgnoreCrossplaneConstraints = b
}

// GetSkipDependencyResolution of this ConfigurationRevision.
func (p *ConfigurationRevision) GetSkipDependencyResolution() *bool {
	return p.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this ConfigurationRevision.
func (p *ConfigurationRevision) SetSkipDependencyResolution(b *bool) {
	p.Spec.SkipDependencyResolution = b
}

// GetCommonLabels of this ConfigurationRevision.
func (p *ConfigurationRevision) GetCommonLabels() map[string]string {
	return p.Spec.CommonLabels
}

// SetCommonLabels of this ConfigurationRevision.
func (p *ConfigurationRevision) SetCommonLabels(l map[string]string) {
	p.Spec.CommonLabels = l
}

// GetAppliedImageConfigRefs of this ConfigurationRevision.
func (p *ConfigurationRevision) GetAppliedImageConfigRefs() []ImageConfigRef {
	return p.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this ConfigurationRevision.
func (p *ConfigurationRevision) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	p.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this ConfigurationRevision.
func (p *ConfigurationRevision) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	p.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this ConfigurationRevision.
func (p *ConfigurationRevision) GetResolvedSource() string {
	return p.Status.ResolvedPackage
}

// SetResolvedSource of this ConfigurationRevision.
func (p *ConfigurationRevision) SetResolvedSource(s string) {
	p.Status.ResolvedPackage = s
}

// PackageRevisionList is the interface satisfied by package revision list
// types.
// +k8s:deepcopy-gen=false
type PackageRevisionList interface {
	client.ObjectList

	// GetRevisions gets the list of PackageRevisions in a PackageRevisionList.
	// This is a costly operation, but allows for treating different revision
	// list types as a single interface. If causing a performance bottleneck in
	// a shared reconciler, consider refactoring the controller to use a
	// reconciler for the specific type.
	GetRevisions() []PackageRevision
}

// GetRevisions of this ProviderRevisionList.
func (p *ProviderRevisionList) GetRevisions() []PackageRevision {
	prs := make([]PackageRevision, len(p.Items))
	for i, r := range p.Items {
		prs[i] = &r
	}
	return prs
}

// GetRevisions of this ConfigurationRevisionList.
func (p *ConfigurationRevisionList) GetRevisions() []PackageRevision {
	prs := make([]PackageRevision, len(p.Items))
	for i, r := range p.Items {
		prs[i] = &r
	}
	return prs
}

const (
	// TLSServerSecretNameSuffix is the suffix added to the name of a secret that
	// contains TLS server certificates.
	TLSServerSecretNameSuffix = "-tls-server"
	// TLSClientSecretNameSuffix is the suffix added to the name of a secret that
	// contains TLS client certificates.
	TLSClientSecretNameSuffix = "-tls-client"
)

// GetSecretNameWithSuffix returns a secret name with the given suffix.
// K8s secret names can be at most 253 characters long, so we truncate the
// name if necessary.
func GetSecretNameWithSuffix(name, suffix string) *string {
	if len(name) > 253-len(suffix) {
		name = name[0 : 253-len(suffix)]
	}
	s := name + suffix

	return &s
}

// GetCondition of this Function.
func (f *Function) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return f.Status.GetCondition(ct)
}

// SetConditions of this Function.
func (f *Function) SetConditions(c ...xpv1.Condition) {
	f.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (f *Function) CleanConditions() {
	f.Status.Conditions = []xpv1.Condition{}
}

// GetSource of this Function.
func (f *Function) GetSource() string {
	return f.Spec.Package
}

// SetSource of this Function.
func (f *Function) SetSource(s string) {
	f.Spec.Package = s
}

// GetActivationPolicy of this Function.
func (f *Function) GetActivationPolicy() *RevisionActivationPolicy {
	return f.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Function.
func (f *Function) SetActivationPolicy(a *RevisionActivationPolicy) {
	f.Spec.RevisionActivationPolicy = a
}

// GetPackagePullSecrets of this Function.
func (f *Function) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return f.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this Function.
func (f *Function) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	f.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this Function.
func (f *Function) GetPackagePullPolicy() *corev1.PullPolicy {
	return f.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this Function.
func (f *Function) SetPackagePullPolicy(i *corev1.PullPolicy) {
	f.Spec.PackagePullPolicy = i
}

// GetRevisionHistoryLimit of this Function.
func (f *Function) GetRevisionHistoryLimit() *int64 {
	return f.Spec.RevisionHistoryLimit
}

// SetRevisionHistoryLimit of this Function.
func (f *Function) SetRevisionHistoryLimit(l *int64) {
	f.Spec.RevisionHistoryLimit = l
}

// GetIgnoreCrossplaneConstraints of this Function.
func (f *Function) GetIgnoreCrossplaneConstraints() *bool {
	return f.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this Function.
func (f *Function) SetIgnoreCrossplaneConstraints(b *bool) {
	f.Spec.IgnoreCrossplaneConstraints = b
}

// GetRuntimeConfigRef of this Function.
func (f *Function) GetRuntimeConfigRef() *RuntimeConfigReference {
	return f.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this Function.
func (f *Function) SetRuntimeConfigRef(r *RuntimeConfigReference) {
	f.Spec.RuntimeConfigReference = r
}

// GetCurrentRevision of this Function.
func (f *Function) GetCurrentRevision() string {
	return f.Status.CurrentRevision
}

// SetCurrentRevision of this Function.
func (f *Function) SetCurrentRevision(s string) {
	f.Status.CurrentRevision = s
}

// GetSkipDependencyResolution of this Function.
func (f *Function) GetSkipDependencyResolution() *bool {
	return f.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this Function.
func (f *Function) SetSkipDependencyResolution(b *bool) {
	f.Spec.SkipDependencyResolution = b
}

// GetCurrentIdentifier of this Function.
func (f *Function) GetCurrentIdentifier() string {
	return f.Status.CurrentIdentifier
}

// SetCurrentIdentifier of this Function.
func (f *Function) SetCurrentIdentifier(s string) {
	f.Status.CurrentIdentifier = s
}

// GetCommonLabels of this Function.
func (f *Function) GetCommonLabels() map[string]string {
	return f.Spec.CommonLabels
}

// SetCommonLabels of this Function.
func (f *Function) SetCommonLabels(l map[string]string) {
	f.Spec.CommonLabels = l
}

// GetTLSServerSecretName of this Function.
func (f *Function) GetTLSServerSecretName() *string {
	return GetSecretNameWithSuffix(f.GetName(), TLSServerSecretNameSuffix)
}

// GetTLSClientSecretName of this Function.
func (f *Function) GetTLSClientSecretName() *string {
	return nil
}

// GetAppliedImageConfigRefs of this Function.
func (f *Function) GetAppliedImageConfigRefs() []ImageConfigRef {
	return f.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this Function.
func (f *Function) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	f.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this Function.
func (f *Function) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	f.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this Function.
func (f *Function) GetResolvedSource() string {
	return f.Status.ResolvedPackage
}

// SetResolvedSource of this Function.
func (f *Function) SetResolvedSource(s string) {
	f.Status.ResolvedPackage = s
}

// GetCondition of this FunctionRevision.
func (r *FunctionRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return r.Status.GetCondition(ct)
}

// SetConditions of this FunctionRevision.
func (r *FunctionRevision) SetConditions(c ...xpv1.Condition) {
	r.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (r *FunctionRevision) CleanConditions() {
	r.Status.Conditions = []xpv1.Condition{}
}

// GetObjects of this FunctionRevision.
func (r *FunctionRevision) GetObjects() []xpv1.TypedReference {
	return r.Status.ObjectRefs
}

// SetObjects of this FunctionRevision.
func (r *FunctionRevision) SetObjects(c []xpv1.TypedReference) {
	r.Status.ObjectRefs = c
}

// GetSource of this FunctionRevision.
func (r *FunctionRevision) GetSource() string {
	return r.Spec.Package
}

// SetSource of this FunctionRevision.
func (r *FunctionRevision) SetSource(s string) {
	r.Spec.Package = s
}

// GetPackagePullSecrets of this FunctionRevision.
func (r *FunctionRevision) GetPackagePullSecrets() []corev1.LocalObjectReference {
	return r.Spec.PackagePullSecrets
}

// SetPackagePullSecrets of this FunctionRevision.
func (r *FunctionRevision) SetPackagePullSecrets(s []corev1.LocalObjectReference) {
	r.Spec.PackagePullSecrets = s
}

// GetPackagePullPolicy of this FunctionRevision.
func (r *FunctionRevision) GetPackagePullPolicy() *corev1.PullPolicy {
	return r.Spec.PackagePullPolicy
}

// SetPackagePullPolicy of this FunctionRevision.
func (r *FunctionRevision) SetPackagePullPolicy(i *corev1.PullPolicy) {
	r.Spec.PackagePullPolicy = i
}

// GetDesiredState of this FunctionRevision.
func (r *FunctionRevision) GetDesiredState() PackageRevisionDesiredState {
	return r.Spec.DesiredState
}

// SetDesiredState of this FunctionRevision.
func (r *FunctionRevision) SetDesiredState(s PackageRevisionDesiredState) {
	r.Spec.DesiredState = s
}

// GetRevision of this FunctionRevision.
func (r *FunctionRevision) GetRevision() int64 {
	return r.Spec.Revision
}

// SetRevision of this FunctionRevision.
func (r *FunctionRevision) SetRevision(rev int64) {
	r.Spec.Revision = rev
}

// GetDependencyStatus of this v.
func (r *FunctionRevision) GetDependencyStatus() (found, installed, invalid int64) {
	return r.Status.FoundDependencies, r.Status.InstalledDependencies, r.Status.InvalidDependencies
}

// SetDependencyStatus of this FunctionRevision.
func (r *FunctionRevision) SetDependencyStatus(found, installed, invalid int64) {
	r.Status.FoundDependencies = found
	r.Status.InstalledDependencies = installed
	r.Status.InvalidDependencies = invalid
}

// GetIgnoreCrossplaneConstraints of this FunctionRevision.
func (r *FunctionRevision) GetIgnoreCrossplaneConstraints() *bool {
	return r.Spec.IgnoreCrossplaneConstraints
}

// SetIgnoreCrossplaneConstraints of this FunctionRevision.
func (r *FunctionRevision) SetIgnoreCrossplaneConstraints(b *bool) {
	r.Spec.IgnoreCrossplaneConstraints = b
}

// GetRuntimeConfigRef of this FunctionRevision.
func (r *FunctionRevision) GetRuntimeConfigRef() *RuntimeConfigReference {
	return r.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this FunctionRevision.
func (r *FunctionRevision) SetRuntimeConfigRef(ref *RuntimeConfigReference) {
	r.Spec.RuntimeConfigReference = ref
}

// GetSkipDependencyResolution of this FunctionRevision.
func (r *FunctionRevision) GetSkipDependencyResolution() *bool {
	return r.Spec.SkipDependencyResolution
}

// SetSkipDependencyResolution of this FunctionRevision.
func (r *FunctionRevision) SetSkipDependencyResolution(b *bool) {
	r.Spec.SkipDependencyResolution = b
}

// GetTLSServerSecretName of this FunctionRevision.
func (r *FunctionRevision) GetTLSServerSecretName() *string {
	return r.Spec.TLSServerSecretName
}

// SetTLSServerSecretName of this FunctionRevision.
func (r *FunctionRevision) SetTLSServerSecretName(s *string) {
	r.Spec.TLSServerSecretName = s
}

// GetTLSClientSecretName of this FunctionRevision.
func (r *FunctionRevision) GetTLSClientSecretName() *string {
	return r.Spec.TLSClientSecretName
}

// SetTLSClientSecretName of this FunctionRevision.
func (r *FunctionRevision) SetTLSClientSecretName(s *string) {
	r.Spec.TLSClientSecretName = s
}

// GetCommonLabels of this FunctionRevision.
func (r *FunctionRevision) GetCommonLabels() map[string]string {
	return r.Spec.CommonLabels
}

// SetCommonLabels of this FunctionRevision.
func (r *FunctionRevision) SetCommonLabels(l map[string]string) {
	r.Spec.CommonLabels = l
}

// GetAppliedImageConfigRefs of this FunctionRevision.
func (r *FunctionRevision) GetAppliedImageConfigRefs() []ImageConfigRef {
	return r.Status.AppliedImageConfigRefs
}

// SetAppliedImageConfigRefs of this FunctionRevision.
func (r *FunctionRevision) SetAppliedImageConfigRefs(refs ...ImageConfigRef) {
	r.Status.SetAppliedImageConfigRefs(refs...)
}

// ClearAppliedImageConfigRef of this FunctionRevision.
func (r *FunctionRevision) ClearAppliedImageConfigRef(reason ImageConfigRefReason) {
	r.Status.ClearAppliedImageConfigRef(reason)
}

// GetResolvedSource of this FunctionRevision.
func (r *FunctionRevision) GetResolvedSource() string {
	return r.Status.ResolvedPackage
}

// SetResolvedSource of this FunctionRevision.
func (r *FunctionRevision) SetResolvedSource(s string) {
	r.Status.ResolvedPackage = s
}

// GetRevisions of this ConfigurationRevisionList.
func (p *FunctionRevisionList) GetRevisions() []PackageRevision {
	prs := make([]PackageRevision, len(p.Items))
	for i, r := range p.Items {
		prs[i] = &r
	}
	return prs
}

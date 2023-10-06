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

// PackageWithRuntime is the interface satisfied by packages with runtime types.
// +k8s:deepcopy-gen=false
type PackageWithRuntime interface {
	Package

	GetControllerConfigRef() *ControllerConfigReference
	SetControllerConfigRef(r *ControllerConfigReference)

	GetRuntimeConfigRef() *RuntimeConfigReference
	SetRuntimeConfigRef(r *RuntimeConfigReference)

	GetTLSServerSecretName() *string

	GetTLSClientSecretName() *string
}

// Package is the interface satisfied by package types.
// +k8s:deepcopy-gen=false
type Package interface {
	resource.Object
	resource.Conditioned

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
	SetSkipDependencyResolution(*bool)

	GetCommonLabels() map[string]string
	SetCommonLabels(l map[string]string)
}

// GetCondition of this Provider.
func (p *Provider) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Provider.
func (p *Provider) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
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

// GetControllerConfigRef of this Provider.
func (p *Provider) GetControllerConfigRef() *ControllerConfigReference {
	return p.Spec.ControllerConfigReference
}

// SetControllerConfigRef of this Provider.
func (p *Provider) SetControllerConfigRef(r *ControllerConfigReference) {
	p.Spec.ControllerConfigReference = r
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

// GetCondition of this Configuration.
func (p *Configuration) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Configuration.
func (p *Configuration) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
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

// PackageRevisionWithRuntime is the interface satisfied by revision of packages
// with runtime types.
// +k8s:deepcopy-gen=false
type PackageRevisionWithRuntime interface {
	PackageRevision

	GetControllerConfigRef() *ControllerConfigReference
	SetControllerConfigRef(r *ControllerConfigReference)

	GetRuntimeConfigRef() *RuntimeConfigReference
	SetRuntimeConfigRef(r *RuntimeConfigReference)

	GetTLSServerSecretName() *string
	SetTLSServerSecretName(n *string)

	GetTLSClientSecretName() *string
	SetTLSClientSecretName(n *string)
}

// PackageRevision is the interface satisfied by package revision types.
// +k8s:deepcopy-gen=false
type PackageRevision interface {
	resource.Object
	resource.Conditioned

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
	SetSkipDependencyResolution(*bool)

	GetDependencyStatus() (found, installed, invalid int64)
	SetDependencyStatus(found, installed, invalid int64)

	GetCommonLabels() map[string]string
	SetCommonLabels(l map[string]string)
}

// GetCondition of this ProviderRevision.
func (p *ProviderRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ProviderRevision.
func (p *ProviderRevision) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
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

// GetControllerConfigRef of this ProviderRevision.
func (p *ProviderRevision) GetControllerConfigRef() *ControllerConfigReference {
	return p.Spec.ControllerConfigReference
}

// SetControllerConfigRef of this ProviderRevision.
func (p *ProviderRevision) SetControllerConfigRef(r *ControllerConfigReference) {
	p.Spec.ControllerConfigReference = r
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

// GetCondition of this ConfigurationRevision.
func (p *ConfigurationRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ConfigurationRevision.
func (p *ConfigurationRevision) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
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
		r := r // Pin range variable so we can take its address.
		prs[i] = &r
	}
	return prs
}

// GetRevisions of this ConfigurationRevisionList.
func (p *ConfigurationRevisionList) GetRevisions() []PackageRevision {
	prs := make([]PackageRevision, len(p.Items))
	for i, r := range p.Items {
		r := r // Pin range variable so we can take its address.
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

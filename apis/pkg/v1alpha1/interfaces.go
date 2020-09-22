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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// RevisionActivationPolicy indicates how a package should activate its
// revisions.
type RevisionActivationPolicy string

const (
	// AutomaticActivation indicates that package should automatically activate
	// package revisions.
	AutomaticActivation RevisionActivationPolicy = "Automatic"
	// ManualActivation indicates that a user will manually activate package
	// revisions.
	ManualActivation RevisionActivationPolicy = "Manual"
)

var _ Package = &Provider{}
var _ Package = &Configuration{}

// Package is the interface satisfied by package types.
// +k8s:deepcopy-gen=false
type Package interface {
	resource.Object
	resource.Conditioned

	GetSource() string
	SetSource(s string)

	GetActivationPolicy() RevisionActivationPolicy
	SetActivationPolicy(a RevisionActivationPolicy)

	GetPackagePullSecrets() []corev1.LocalObjectReference
	SetPackagePullSecrets(s []corev1.LocalObjectReference)

	GetPackagePullPolicy() *corev1.PullPolicy
	SetPackagePullPolicy(i *corev1.PullPolicy)

	GetRevisionHistoryLimit() int64
	SetRevisionHistoryLimit(l int64)

	GetCurrentRevision() string
	SetCurrentRevision(r string)
}

// GetCondition of this Provider.
func (p *Provider) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Provider.
func (p *Provider) SetConditions(c ...runtimev1alpha1.Condition) {
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
func (p *Provider) GetActivationPolicy() RevisionActivationPolicy {
	if p.Spec.RevisionActivationPolicy == nil {
		return AutomaticActivation
	}
	return *p.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Provider.
func (p *Provider) SetActivationPolicy(a RevisionActivationPolicy) {
	p.Spec.RevisionActivationPolicy = &a
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
func (p *Provider) GetRevisionHistoryLimit() int64 {
	if p.Spec.RevisionHistoryLimit == nil {
		return 1
	}
	return *p.Spec.RevisionHistoryLimit
}

// SetRevisionHistoryLimit of this Provider.
func (p *Provider) SetRevisionHistoryLimit(l int64) {
	p.Spec.RevisionHistoryLimit = &l
}

// GetCurrentRevision of this Provider.
func (p *Provider) GetCurrentRevision() string {
	return p.Status.CurrentRevision
}

// SetCurrentRevision of this Provider.
func (p *Provider) SetCurrentRevision(s string) {
	p.Status.CurrentRevision = s
}

// GetCondition of this Configuration.
func (p *Configuration) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this Configuration.
func (p *Configuration) SetConditions(c ...runtimev1alpha1.Condition) {
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
func (p *Configuration) GetActivationPolicy() RevisionActivationPolicy {
	if p.Spec.RevisionActivationPolicy == nil {
		return AutomaticActivation
	}
	return *p.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Configuration.
func (p *Configuration) SetActivationPolicy(a RevisionActivationPolicy) {
	p.Spec.RevisionActivationPolicy = &a
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
func (p *Configuration) GetRevisionHistoryLimit() int64 {
	if p.Spec.RevisionHistoryLimit == nil {
		return 1
	}
	return *p.Spec.RevisionHistoryLimit
}

// SetRevisionHistoryLimit of this Configuration.
func (p *Configuration) SetRevisionHistoryLimit(l int64) {
	p.Spec.RevisionHistoryLimit = &l
}

// GetCurrentRevision of this Configuration.
func (p *Configuration) GetCurrentRevision() string {
	return p.Status.CurrentRevision
}

// SetCurrentRevision of this Configuration.
func (p *Configuration) SetCurrentRevision(s string) {
	p.Status.CurrentRevision = s
}

var _ PackageRevision = &ProviderRevision{}
var _ PackageRevision = &ConfigurationRevision{}

// PackageRevision is the interface satisfied by package revision types.
// +k8s:deepcopy-gen=false
type PackageRevision interface {
	resource.Object
	resource.Conditioned

	GetObjects() []runtimev1alpha1.TypedReference
	SetObjects(c []runtimev1alpha1.TypedReference)

	GetControllerReference() runtimev1alpha1.Reference
	SetControllerReference(c runtimev1alpha1.Reference)

	GetSource() string
	SetSource(s string)

	GetDesiredState() PackageRevisionDesiredState
	SetDesiredState(d PackageRevisionDesiredState)

	SetInstallPod(j runtimev1alpha1.Reference)
	GetInstallPod() runtimev1alpha1.Reference

	GetRevision() int64
	SetRevision(r int64)
}

// GetCondition of this ProviderRevision.
func (p *ProviderRevision) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ProviderRevision.
func (p *ProviderRevision) SetConditions(c ...runtimev1alpha1.Condition) {
	p.Status.SetConditions(c...)
}

// GetObjects of this ProviderRevision.
func (p *ProviderRevision) GetObjects() []runtimev1alpha1.TypedReference {
	return p.Status.ObjectRefs
}

// SetObjects of this ProviderRevision.
func (p *ProviderRevision) SetObjects(c []runtimev1alpha1.TypedReference) {
	p.Status.ObjectRefs = c
}

// GetControllerReference of this ProviderRevision.
func (p *ProviderRevision) GetControllerReference() runtimev1alpha1.Reference {
	return p.Status.ControllerRef
}

// SetControllerReference of this ProviderRevision.
func (p *ProviderRevision) SetControllerReference(c runtimev1alpha1.Reference) {
	p.Status.ControllerRef = c
}

// GetSource of this ProviderRevision.
func (p *ProviderRevision) GetSource() string {
	return p.Spec.Image
}

// SetSource of this ProviderRevision.
func (p *ProviderRevision) SetSource(s string) {
	p.Spec.Image = s
}

// GetDesiredState of this ProviderRevision.
func (p *ProviderRevision) GetDesiredState() PackageRevisionDesiredState {
	return p.Spec.DesiredState
}

// SetDesiredState of this ProviderRevision.
func (p *ProviderRevision) SetDesiredState(s PackageRevisionDesiredState) {
	p.Spec.DesiredState = s
}

// GetInstallPod of this ProviderRevision.
func (p *ProviderRevision) GetInstallPod() runtimev1alpha1.Reference {
	return p.Spec.InstallPodRef
}

// SetInstallPod of this ProviderRevision.
func (p *ProviderRevision) SetInstallPod(j runtimev1alpha1.Reference) {
	p.Spec.InstallPodRef = j
}

// GetRevision of this ProviderRevision.
func (p *ProviderRevision) GetRevision() int64 {
	return p.Spec.Revision
}

// SetRevision of this ProviderRevision.
func (p *ProviderRevision) SetRevision(r int64) {
	p.Spec.Revision = r
}

// GetCondition of this ConfigurationRevision.
func (p *ConfigurationRevision) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ConfigurationRevision.
func (p *ConfigurationRevision) SetConditions(c ...runtimev1alpha1.Condition) {
	p.Status.SetConditions(c...)
}

// GetObjects of this ConfigurationRevision.
func (p *ConfigurationRevision) GetObjects() []runtimev1alpha1.TypedReference {
	return p.Status.ObjectRefs
}

// SetObjects of this ConfigurationRevision.
func (p *ConfigurationRevision) SetObjects(c []runtimev1alpha1.TypedReference) {
	p.Status.ObjectRefs = c
}

// GetControllerReference of this ConfigurationRevision.
func (p *ConfigurationRevision) GetControllerReference() runtimev1alpha1.Reference {
	return p.Status.ControllerRef
}

// SetControllerReference of this ConfigurationRevision.
func (p *ConfigurationRevision) SetControllerReference(c runtimev1alpha1.Reference) {
	p.Status.ControllerRef = c
}

// GetSource of this ConfigurationRevision.
func (p *ConfigurationRevision) GetSource() string {
	return p.Spec.Image
}

// SetSource of this ConfigurationRevision.
func (p *ConfigurationRevision) SetSource(s string) {
	p.Spec.Image = s
}

// GetDesiredState of this ConfigurationRevision.
func (p *ConfigurationRevision) GetDesiredState() PackageRevisionDesiredState {
	return p.Spec.DesiredState
}

// SetDesiredState of this ConfigurationRevision.
func (p *ConfigurationRevision) SetDesiredState(s PackageRevisionDesiredState) {
	p.Spec.DesiredState = s
}

// GetInstallPod of this ConfigurationRevision.
func (p *ConfigurationRevision) GetInstallPod() runtimev1alpha1.Reference {
	return p.Spec.InstallPodRef
}

// SetInstallPod of this ConfigurationRevision.
func (p *ConfigurationRevision) SetInstallPod(j runtimev1alpha1.Reference) {
	p.Spec.InstallPodRef = j
}

// GetRevision of this ConfigurationRevision.
func (p *ConfigurationRevision) GetRevision() int64 {
	return p.Spec.Revision
}

// SetRevision of this ConfigurationRevision.
func (p *ConfigurationRevision) SetRevision(r int64) {
	p.Spec.Revision = r
}

var _ PackageRevisionList = &ProviderRevisionList{}
var _ PackageRevisionList = &ConfigurationRevisionList{}

// PackageRevisionList is the interface satisfied by package revision list
// types.
// +k8s:deepcopy-gen=false
type PackageRevisionList interface {
	runtime.Object

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

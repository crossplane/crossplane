/*
Copyright 2023 The Crossplane Authors.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// GetCondition of this Function.
func (f *Function) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return f.Status.GetCondition(ct)
}

// SetConditions of this Function.
func (f *Function) SetConditions(c ...xpv1.Condition) {
	f.Status.SetConditions(c...)
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
func (f *Function) GetActivationPolicy() *v1.RevisionActivationPolicy {
	return f.Spec.RevisionActivationPolicy
}

// SetActivationPolicy of this Function.
func (f *Function) SetActivationPolicy(a *v1.RevisionActivationPolicy) {
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

// GetControllerConfigRef of this Function.
func (f *Function) GetControllerConfigRef() *v1.ControllerConfigReference {
	return nil
}

// SetControllerConfigRef of this Function.
func (f *Function) SetControllerConfigRef(*v1.ControllerConfigReference) {}

// GetRuntimeConfigRef of this Function.
func (f *Function) GetRuntimeConfigRef() *v1.RuntimeConfigReference {
	return f.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this Function.
func (f *Function) SetRuntimeConfigRef(r *v1.RuntimeConfigReference) {
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
	return v1.GetSecretNameWithSuffix(f.GetName(), v1.TLSServerSecretNameSuffix)
}

// GetTLSClientSecretName of this Function.
func (f *Function) GetTLSClientSecretName() *string {
	return nil
}

// GetCondition of this FunctionRevision.
func (r *FunctionRevision) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return r.Status.GetCondition(ct)
}

// SetConditions of this FunctionRevision.
func (r *FunctionRevision) SetConditions(c ...xpv1.Condition) {
	r.Status.SetConditions(c...)
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
func (r *FunctionRevision) GetDesiredState() v1.PackageRevisionDesiredState {
	return r.Spec.DesiredState
}

// SetDesiredState of this FunctionRevision.
func (r *FunctionRevision) SetDesiredState(s v1.PackageRevisionDesiredState) {
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

// GetControllerConfigRef of this FunctionRevision.
func (r *FunctionRevision) GetControllerConfigRef() *v1.ControllerConfigReference {
	return r.Spec.ControllerConfigReference
}

// SetControllerConfigRef of this FunctionRevision.
func (r *FunctionRevision) SetControllerConfigRef(ref *v1.ControllerConfigReference) {
	r.Spec.ControllerConfigReference = ref
}

// GetRuntimeConfigRef of this FunctionRevision.
func (r *FunctionRevision) GetRuntimeConfigRef() *v1.RuntimeConfigReference {
	return r.Spec.RuntimeConfigReference
}

// SetRuntimeConfigRef of this FunctionRevision.
func (r *FunctionRevision) SetRuntimeConfigRef(ref *v1.RuntimeConfigReference) {
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

// GetRevisions of this ConfigurationRevisionList.
func (p *FunctionRevisionList) GetRevisions() []v1.PackageRevision {
	prs := make([]v1.PackageRevision, len(p.Items))
	for i, r := range p.Items {
		r := r // Pin range variable so we can take its address.
		prs[i] = &r
	}
	return prs
}

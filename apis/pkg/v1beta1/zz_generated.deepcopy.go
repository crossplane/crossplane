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
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerConfigReference) DeepCopyInto(out *ControllerConfigReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerConfigReference.
func (in *ControllerConfigReference) DeepCopy() *ControllerConfigReference {
	if in == nil {
		return nil
	}
	out := new(ControllerConfigReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerReference) DeepCopyInto(out *ControllerReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerReference.
func (in *ControllerReference) DeepCopy() *ControllerReference {
	if in == nil {
		return nil
	}
	out := new(ControllerReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Dependency) DeepCopyInto(out *Dependency) {
	*out = *in
	if in.ParentConstraints != nil {
		in, out := &in.ParentConstraints, &out.ParentConstraints
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Dependency.
func (in *Dependency) DeepCopy() *Dependency {
	if in == nil {
		return nil
	}
	out := new(Dependency)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentRuntimeConfig) DeepCopyInto(out *DeploymentRuntimeConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentRuntimeConfig.
func (in *DeploymentRuntimeConfig) DeepCopy() *DeploymentRuntimeConfig {
	if in == nil {
		return nil
	}
	out := new(DeploymentRuntimeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DeploymentRuntimeConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentRuntimeConfigList) DeepCopyInto(out *DeploymentRuntimeConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DeploymentRuntimeConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentRuntimeConfigList.
func (in *DeploymentRuntimeConfigList) DeepCopy() *DeploymentRuntimeConfigList {
	if in == nil {
		return nil
	}
	out := new(DeploymentRuntimeConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DeploymentRuntimeConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentRuntimeConfigSpec) DeepCopyInto(out *DeploymentRuntimeConfigSpec) {
	*out = *in
	if in.DeploymentTemplate != nil {
		in, out := &in.DeploymentTemplate, &out.DeploymentTemplate
		*out = new(DeploymentTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceTemplate != nil {
		in, out := &in.ServiceTemplate, &out.ServiceTemplate
		*out = new(ServiceTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceAccountTemplate != nil {
		in, out := &in.ServiceAccountTemplate, &out.ServiceAccountTemplate
		*out = new(ServiceAccountTemplate)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentRuntimeConfigSpec.
func (in *DeploymentRuntimeConfigSpec) DeepCopy() *DeploymentRuntimeConfigSpec {
	if in == nil {
		return nil
	}
	out := new(DeploymentRuntimeConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentTemplate) DeepCopyInto(out *DeploymentTemplate) {
	*out = *in
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(v1.DeploymentSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentTemplate.
func (in *DeploymentTemplate) DeepCopy() *DeploymentTemplate {
	if in == nil {
		return nil
	}
	out := new(DeploymentTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Function) DeepCopyInto(out *Function) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Function.
func (in *Function) DeepCopy() *Function {
	if in == nil {
		return nil
	}
	out := new(Function)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Function) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionList) DeepCopyInto(out *FunctionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Function, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionList.
func (in *FunctionList) DeepCopy() *FunctionList {
	if in == nil {
		return nil
	}
	out := new(FunctionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FunctionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionRevision) DeepCopyInto(out *FunctionRevision) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionRevision.
func (in *FunctionRevision) DeepCopy() *FunctionRevision {
	if in == nil {
		return nil
	}
	out := new(FunctionRevision)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FunctionRevision) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionRevisionList) DeepCopyInto(out *FunctionRevisionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FunctionRevision, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionRevisionList.
func (in *FunctionRevisionList) DeepCopy() *FunctionRevisionList {
	if in == nil {
		return nil
	}
	out := new(FunctionRevisionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FunctionRevisionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionRevisionSpec) DeepCopyInto(out *FunctionRevisionSpec) {
	*out = *in
	in.PackageRevisionSpec.DeepCopyInto(&out.PackageRevisionSpec)
	in.PackageRevisionRuntimeSpec.DeepCopyInto(&out.PackageRevisionRuntimeSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionRevisionSpec.
func (in *FunctionRevisionSpec) DeepCopy() *FunctionRevisionSpec {
	if in == nil {
		return nil
	}
	out := new(FunctionRevisionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionRevisionStatus) DeepCopyInto(out *FunctionRevisionStatus) {
	*out = *in
	in.PackageRevisionStatus.DeepCopyInto(&out.PackageRevisionStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionRevisionStatus.
func (in *FunctionRevisionStatus) DeepCopy() *FunctionRevisionStatus {
	if in == nil {
		return nil
	}
	out := new(FunctionRevisionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionSpec) DeepCopyInto(out *FunctionSpec) {
	*out = *in
	in.PackageSpec.DeepCopyInto(&out.PackageSpec)
	in.PackageRuntimeSpec.DeepCopyInto(&out.PackageRuntimeSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionSpec.
func (in *FunctionSpec) DeepCopy() *FunctionSpec {
	if in == nil {
		return nil
	}
	out := new(FunctionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FunctionStatus) DeepCopyInto(out *FunctionStatus) {
	*out = *in
	in.ConditionedStatus.DeepCopyInto(&out.ConditionedStatus)
	out.PackageStatus = in.PackageStatus
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FunctionStatus.
func (in *FunctionStatus) DeepCopy() *FunctionStatus {
	if in == nil {
		return nil
	}
	out := new(FunctionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Lock) DeepCopyInto(out *Lock) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Packages != nil {
		in, out := &in.Packages, &out.Packages
		*out = make([]LockPackage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Lock.
func (in *Lock) DeepCopy() *Lock {
	if in == nil {
		return nil
	}
	out := new(Lock)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Lock) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LockList) DeepCopyInto(out *LockList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Lock, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LockList.
func (in *LockList) DeepCopy() *LockList {
	if in == nil {
		return nil
	}
	out := new(LockList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LockList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LockPackage) DeepCopyInto(out *LockPackage) {
	*out = *in
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]Dependency, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ParentConstraints != nil {
		in, out := &in.ParentConstraints, &out.ParentConstraints
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LockPackage.
func (in *LockPackage) DeepCopy() *LockPackage {
	if in == nil {
		return nil
	}
	out := new(LockPackage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMeta.
func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRevisionRuntimeSpec) DeepCopyInto(out *PackageRevisionRuntimeSpec) {
	*out = *in
	in.PackageRuntimeSpec.DeepCopyInto(&out.PackageRuntimeSpec)
	if in.TLSServerSecretName != nil {
		in, out := &in.TLSServerSecretName, &out.TLSServerSecretName
		*out = new(string)
		**out = **in
	}
	if in.TLSClientSecretName != nil {
		in, out := &in.TLSClientSecretName, &out.TLSClientSecretName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRevisionRuntimeSpec.
func (in *PackageRevisionRuntimeSpec) DeepCopy() *PackageRevisionRuntimeSpec {
	if in == nil {
		return nil
	}
	out := new(PackageRevisionRuntimeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRevisionSpec) DeepCopyInto(out *PackageRevisionSpec) {
	*out = *in
	if in.PackagePullSecrets != nil {
		in, out := &in.PackagePullSecrets, &out.PackagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.PackagePullPolicy != nil {
		in, out := &in.PackagePullPolicy, &out.PackagePullPolicy
		*out = new(corev1.PullPolicy)
		**out = **in
	}
	if in.IgnoreCrossplaneConstraints != nil {
		in, out := &in.IgnoreCrossplaneConstraints, &out.IgnoreCrossplaneConstraints
		*out = new(bool)
		**out = **in
	}
	if in.SkipDependencyResolution != nil {
		in, out := &in.SkipDependencyResolution, &out.SkipDependencyResolution
		*out = new(bool)
		**out = **in
	}
	if in.CommonLabels != nil {
		in, out := &in.CommonLabels, &out.CommonLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRevisionSpec.
func (in *PackageRevisionSpec) DeepCopy() *PackageRevisionSpec {
	if in == nil {
		return nil
	}
	out := new(PackageRevisionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRevisionStatus) DeepCopyInto(out *PackageRevisionStatus) {
	*out = *in
	in.ConditionedStatus.DeepCopyInto(&out.ConditionedStatus)
	if in.ObjectRefs != nil {
		in, out := &in.ObjectRefs, &out.ObjectRefs
		*out = make([]commonv1.TypedReference, len(*in))
		copy(*out, *in)
	}
	if in.PermissionRequests != nil {
		in, out := &in.PermissionRequests, &out.PermissionRequests
		*out = make([]rbacv1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRevisionStatus.
func (in *PackageRevisionStatus) DeepCopy() *PackageRevisionStatus {
	if in == nil {
		return nil
	}
	out := new(PackageRevisionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRuntimeSpec) DeepCopyInto(out *PackageRuntimeSpec) {
	*out = *in
	if in.ControllerConfigReference != nil {
		in, out := &in.ControllerConfigReference, &out.ControllerConfigReference
		*out = new(ControllerConfigReference)
		**out = **in
	}
	if in.RuntimeConfigReference != nil {
		in, out := &in.RuntimeConfigReference, &out.RuntimeConfigReference
		*out = new(RuntimeConfigReference)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRuntimeSpec.
func (in *PackageRuntimeSpec) DeepCopy() *PackageRuntimeSpec {
	if in == nil {
		return nil
	}
	out := new(PackageRuntimeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageSpec) DeepCopyInto(out *PackageSpec) {
	*out = *in
	if in.RevisionActivationPolicy != nil {
		in, out := &in.RevisionActivationPolicy, &out.RevisionActivationPolicy
		*out = new(RevisionActivationPolicy)
		**out = **in
	}
	if in.RevisionHistoryLimit != nil {
		in, out := &in.RevisionHistoryLimit, &out.RevisionHistoryLimit
		*out = new(int64)
		**out = **in
	}
	if in.PackagePullSecrets != nil {
		in, out := &in.PackagePullSecrets, &out.PackagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.PackagePullPolicy != nil {
		in, out := &in.PackagePullPolicy, &out.PackagePullPolicy
		*out = new(corev1.PullPolicy)
		**out = **in
	}
	if in.IgnoreCrossplaneConstraints != nil {
		in, out := &in.IgnoreCrossplaneConstraints, &out.IgnoreCrossplaneConstraints
		*out = new(bool)
		**out = **in
	}
	if in.SkipDependencyResolution != nil {
		in, out := &in.SkipDependencyResolution, &out.SkipDependencyResolution
		*out = new(bool)
		**out = **in
	}
	if in.CommonLabels != nil {
		in, out := &in.CommonLabels, &out.CommonLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageSpec.
func (in *PackageSpec) DeepCopy() *PackageSpec {
	if in == nil {
		return nil
	}
	out := new(PackageSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageStatus) DeepCopyInto(out *PackageStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageStatus.
func (in *PackageStatus) DeepCopy() *PackageStatus {
	if in == nil {
		return nil
	}
	out := new(PackageStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuntimeConfigReference) DeepCopyInto(out *RuntimeConfigReference) {
	*out = *in
	if in.APIVersion != nil {
		in, out := &in.APIVersion, &out.APIVersion
		*out = new(string)
		**out = **in
	}
	if in.Kind != nil {
		in, out := &in.Kind, &out.Kind
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuntimeConfigReference.
func (in *RuntimeConfigReference) DeepCopy() *RuntimeConfigReference {
	if in == nil {
		return nil
	}
	out := new(RuntimeConfigReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceAccountTemplate) DeepCopyInto(out *ServiceAccountTemplate) {
	*out = *in
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceAccountTemplate.
func (in *ServiceAccountTemplate) DeepCopy() *ServiceAccountTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceTemplate) DeepCopyInto(out *ServiceTemplate) {
	*out = *in
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceTemplate.
func (in *ServiceTemplate) DeepCopy() *ServiceTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceTemplate)
	in.DeepCopyInto(out)
	return out
}

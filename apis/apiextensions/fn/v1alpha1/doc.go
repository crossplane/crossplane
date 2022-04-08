/*
Copyright 2022 The Crossplane Authors.

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

// TODO(negz): If we want to be compatible with KRM functions - i.e. be a true
// superset of KRM functions we should probably use config.kubernetes.io per
// https://github.com/kubernetes-sigs/kustomize/blob/master/cmd/config/docs/api-conventions/functions-spec.md

// Package v1alpha1 contains meta types used to invoke XRM functions.
// +kubebuilder:object:generate=true
// +groupName=fn.apiextensions.crossplane.io
// +versionName=v1alpha1
package v1alpha1

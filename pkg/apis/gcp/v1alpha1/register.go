/*
Copyright 2018 The Crossplane Authors.

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


// Package v1alpha1 contains API Schema definitions for the gcp v1alpha1 API group
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/crossplaneio/crossplane/pkg/gcp/apis/gcp
// +k8s:defaulter-gen=TypeMeta
// +groupName=gcp.crossplane.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

// Kubernetes Group, Version, and Kind metadata.
const (
	Group      = "gcp.crossplane.io"
	Version    = "v1alpha1"
	APIVersion = Group + "/" + Version

	ProviderKind           = "provider"
	ProviderKindAPIVersion = ProviderKind + "." + APIVersion
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// ProviderGroupVersionKind is the GVK of a Provider.
	ProviderGroupVersionKind = schema.GroupVersionKind{
		Group:   Group,
		Version: Version,
		Kind:    ProviderKind,
	}
)

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

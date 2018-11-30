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

// NOTE: Boilerplate only.  Ignore this file.

// Package v1alpha1 contains API Schema definitions for the container v1alpha1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/crossplaneio/crossplane/pkg/azure/apis/azure/compute
// +k8s:defaulter-gen=TypeMeta
// +groupName=compute.azure.crossplane.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

const (
	// Group is the API Group for these resources
	Group = "compute.azure.crossplane.io"
	// Version is the version of the API group
	Version = "v1alpha1"
	// APIVersion is the full version of this API group
	APIVersion = Group + "/" + Version
	// AKSClusterKind is the kind for the AKS cluster resource
	AKSClusterKind = "akscluster"
	// AKSClusterKindAPIVersion is the full kind and version for the AKS cluster resource
	AKSClusterKindAPIVersion = AKSClusterKind + "." + APIVersion
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

func init() {
	SchemeBuilder.Register(&AKSCluster{}, &AKSClusterList{})
}

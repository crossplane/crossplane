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

package v1

// PackageRuntimeSpec specifies configuration for the runtime of a package.
// Only used by packages that uses a runtime, i.e. by providers and functions
// but not for configurations.
type PackageRuntimeSpec struct {
	// ControllerConfigRef references a ControllerConfig resource that will be
	// used to configure the packaged controller Deployment.
	// Deprecated: Use RuntimeConfigReference instead.
	// +optional
	ControllerConfigReference *ControllerConfigReference `json:"controllerConfigRef,omitempty"`
}

// PackageRevisionRuntimeSpec specifies configuration for the runtime of a
// package revision. Only used by packages that uses a runtime, i.e. by
// providers and functions but not for configurations.
type PackageRevisionRuntimeSpec struct {
	PackageRuntimeSpec `json:",inline"`
	// TLSServerSecretName is the name of the TLS Secret that stores server
	// certificates of the Provider.
	// +optional
	TLSServerSecretName *string `json:"tlsServerSecretName,omitempty"`

	// TLSClientSecretName is the name of the TLS Secret that stores client
	// certificates of the Provider.
	// +optional
	TLSClientSecretName *string `json:"tlsClientSecretName,omitempty"`
}

// A ControllerConfigReference to a ControllerConfig resource that will be used
// to configure the packaged controller Deployment.
type ControllerConfigReference struct {
	// Name of the ControllerConfig.
	Name string `json:"name"`
}

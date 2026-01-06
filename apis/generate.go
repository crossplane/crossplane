//go:build generate

/*
Copyright 2026 The Crossplane Authors.

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

// Code generation tools (controller-gen, goverter, buf, etc.) must be in your
// $PATH. Use './nix.sh develop' or './nix.sh run .#generate' to ensure they
// are.

// Remove existing manifests
//go:generate rm -rf ../cluster/crds
//go:generate rm -rf ../cluster/webhookconfigurations/manifests.yaml

// Replicate identical API versions

//go:generate ./hack/duplicate_api_type.sh apiextensions/v1beta1/usage_types.go apiextensions/v1alpha1 true

//go:generate ./hack/duplicate_api_type.sh pkg/v1/package_types.go pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh pkg/v1/package_runtime_types.go pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh pkg/v1/revision_types.go pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh pkg/v1/function_types.go pkg/v1beta1

//go:generate ./hack/duplicate_api_type.sh pkg/meta/v1/configuration_types.go pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh pkg/meta/v1/provider_types.go pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh pkg/meta/v1/function_types.go pkg/meta/v1beta1
//go:generate ./hack/duplicate_api_type.sh pkg/meta/v1/meta.go pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh pkg/meta/v1/meta.go pkg/meta/v1beta1

// NOTE(negz): We generate deepcopy methods and CRDs for each API group
// separately because there seems to be an undiagnosed bug in controller-runtime
// that causes some kubebuilder annotations to be ignored when we try to
// generate them all together in one command.

// Generate deepcopy methodsets and CRD manifests
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./common/v1;./common/v2
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./pkg/v1beta1;./pkg/v1 crd:crdVersions=v1,generateEmbeddedObjectMeta=true output:artifacts:config=../cluster/crds
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./apiextensions/v1alpha1;./apiextensions/v1beta1;./apiextensions/v1;./apiextensions/v2 crd:crdVersions=v1 output:artifacts:config=../cluster/crds
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./protection/v1beta1 crd:crdVersions=v1 output:artifacts:config=../cluster/crds
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./ops/v1alpha1 crd:crdVersions=v1 output:artifacts:config=../cluster/crds

// We generate the meta.pkg.crossplane.io types separately as the generated CRDs
// are never installed, only used for API documentation.
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./pkg/meta/... crd:crdVersions=v1 output:artifacts:config=../cluster/meta

// Generate webhook manifests
//go:generate controller-gen webhook paths=./pkg/v1beta1;./pkg/v1;./apiextensions/v1alpha1;./apiextensions/v1beta1;./apiextensions/v1;./protection/v1beta1 output:artifacts:config=../cluster/webhookconfigurations

// Generate conversion code
//go:generate goverter gen -build-tags="" ./apiextensions/v1
//go:generate goverter gen -build-tags="" ./pkg/meta/v1alpha1
//go:generate goverter gen -build-tags="" ./pkg/meta/v1beta1

package generate

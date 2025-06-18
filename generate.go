//go:build generate
// +build generate

/*
Copyright 2019 The Crossplane Authors.

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

// NOTE(negz): See the below link for details on what is happening here.
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Remove existing manifests
//go:generate rm -rf ./cluster/crds
//go:generate rm -rf ./cluster/webhookconfigurations/manifests.yaml

// Replicate identical API versions

//go:generate ./hack/duplicate_api_type.sh apis/apiextensions/v1beta1/usage_types.go apis/apiextensions/v1alpha1 true

//go:generate ./hack/duplicate_api_type.sh apis/pkg/v1/package_types.go apis/pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/v1/package_runtime_types.go apis/pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/v1/revision_types.go apis/pkg/v1beta1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/v1/function_types.go apis/pkg/v1beta1

//go:generate ./hack/duplicate_api_type.sh apis/pkg/meta/v1/configuration_types.go apis/pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/meta/v1/provider_types.go apis/pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/meta/v1/function_types.go apis/pkg/meta/v1beta1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/meta/v1/meta.go apis/pkg/meta/v1alpha1
//go:generate ./hack/duplicate_api_type.sh apis/pkg/meta/v1/meta.go apis/pkg/meta/v1beta1

// NOTE(negz): We generate deepcopy methods and CRDs for each API group
// separately because there seems to be an undiagnosed bug in controller-runtime
// that causes some kubebuilder annotations to be ignored when we try to
// generate them all together in one command.

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./apis/pkg/v1beta1;./apis/pkg/v1 crd:crdVersions=v1,generateEmbeddedObjectMeta=true output:artifacts:config=./cluster/crds
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./apis/apiextensions/v1alpha1;./apis/apiextensions/v1beta1;./apis/apiextensions/v1;./apis/apiextensions/v2alpha1;./apis/apiextensions/v2 crd:crdVersions=v1 output:artifacts:config=./cluster/crds
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./apis/protection/v1beta1 crd:crdVersions=v1 output:artifacts:config=./cluster/crds

// We generate the meta.pkg.crossplane.io types separately as the generated CRDs
// are never installed, only used for API documentation.
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./apis/pkg/meta/... crd:crdVersions=v1 output:artifacts:config=./cluster/meta

// Generate webhook manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen webhook paths=./apis/pkg/v1beta1;./apis/pkg/v1;./apis/apiextensions/v1alpha1;./apis/apiextensions/v1beta1;./apis/apiextensions/v1;./apis/protection/v1beta1 output:artifacts:config=./cluster/webhookconfigurations

// Generate conversion code
//go:generate go run -tags generate github.com/jmattheis/goverter/cmd/goverter gen -build-tags="" ./apis/apiextensions/v1
//go:generate go run -tags generate github.com/jmattheis/goverter/cmd/goverter gen -build-tags="" ./apis/protection/v1beta1
//go:generate go run -tags generate github.com/jmattheis/goverter/cmd/goverter gen -build-tags="" ./apis/pkg/meta/v1alpha1
//go:generate go run -tags generate github.com/jmattheis/goverter/cmd/goverter gen -build-tags="" ./apis/pkg/meta/v1beta1

// Replicate identical gRPC APIs

//go:generate ./hack/duplicate_proto_type.sh apis/apiextensions/fn/proto/v1/run_function.proto apis/apiextensions/fn/proto/v1beta1

// Generate gRPC types and stubs.
//
// We use buf rather than the traditional protoc because it's pure go and can
// thus be invoked using go run from a pinned dependency. If we used protoc we'd
// need to install it via the Makefile, and there are not currently statically
// compiled binaries available for download (the release binaries for Linux are
// dynamically linked). See buf.gen.yaml for buf's configuration.
//
// We go install the required plugins because they need to be in $PATH for buf
// (or protoc) to invoke them.

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc
//go:generate go run github.com/bufbuild/buf/cmd/buf@v1.53.0 generate

package generate

import (
	_ "github.com/jmattheis/goverter/cmd/goverter"      //nolint:typecheck
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"   //nolint:typecheck
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"    //nolint:typecheck
	_ "k8s.io/code-generator"                           //nolint:typecheck
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck
)

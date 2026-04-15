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

// Code generation tools (controller-gen, goverter, buf, etc.) must be in your
// $PATH. Use './nix.sh develop' or './nix.sh run .#generate' to ensure they
// are.

// Generate deepcopy methodsets for internal APIs
//go:generate controller-gen object:headerFile=./hack/boilerplate.go.txt paths=./internal/protection

// Generate conversion code
//go:generate goverter gen -build-tags="" ./internal/protection

// Replicate identical gRPC APIs

//go:generate ./hack/duplicate_proto_type.sh proto/fn/v1/run_function.proto proto/fn/v1beta1

// Generate gRPC types and stubs. See buf.gen.yaml for buf's configuration.
// The protoc-gen-go and protoc-gen-go-grpc plugins must be in $PATH.
// Note that the vendor dir does temporarily exist during a Nix build.
//go:generate buf generate --exclude-path vendor

package generate

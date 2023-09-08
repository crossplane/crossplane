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
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Generate deepcopy methodsets
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./...

// Generate External Secret Store gRPC types and stubs.
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
//go:generate go run github.com/bufbuild/buf/cmd/buf generate

// Package apis contains Kubernetes API groups
package apis

import (
	_ "github.com/bufbuild/buf/cmd/buf"                 //nolint:typecheck
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"   //nolint:typecheck
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"    //nolint:typecheck
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck
)

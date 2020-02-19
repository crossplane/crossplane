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

// Remove existing CRDs
//go:generate rm -rf ../cluster/charts/crossplane-types/crds

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:trivialVersions=true output:artifacts:config=../cluster/charts/crossplane-types/crds

// Re-generate stack CRD manifests without descriptions.
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen paths=./stacks/... crd:maxDescLen=0,trivialVersions=true output:artifacts:config=../cluster/charts/crossplane-types/crds

// Generate crossplane-runtime methodsets (resource.Claim, etc)
//go:generate go run -tags generate github.com/crossplane/crossplane-tools/cmd/angryjet generate-methodsets --header-file=../hack/boilerplate.go.txt ./...

package apis

import (
	_ "github.com/crossplane/crossplane-tools/cmd/angryjet" //nolint:typecheck
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"     //nolint:typecheck
)

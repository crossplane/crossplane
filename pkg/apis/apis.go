/*
Copyright 2018 The Conductor Authors.

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

// Generate deepcopy for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./... -h ../../hack/boilerplate.go.txt

// Package apis contains Kubernetes API groups
package apis

import (
	"github.com/upbound/conductor/pkg/apis/aws"
	"github.com/upbound/conductor/pkg/apis/azure"
	"github.com/upbound/conductor/pkg/apis/compute"
	"github.com/upbound/conductor/pkg/apis/core"
	"github.com/upbound/conductor/pkg/apis/gcp"
	"github.com/upbound/conductor/pkg/apis/storage"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, aws.AddToScheme)
	AddToSchemes = append(AddToSchemes, azure.AddToScheme)
	AddToSchemes = append(AddToSchemes, compute.AddToScheme)
	AddToSchemes = append(AddToSchemes, core.AddToScheme)
	AddToSchemes = append(AddToSchemes, gcp.AddToScheme)
	AddToSchemes = append(AddToSchemes, storage.AddToScheme)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

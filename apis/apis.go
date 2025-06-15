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

// Package apis contains Kubernetes API groups
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/apiextensions"
	"github.com/crossplane/crossplane/apis/pkg"
	"github.com/crossplane/crossplane/apis/protection"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		apiextensions.AddToScheme,
		pkg.AddToScheme,
		protection.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

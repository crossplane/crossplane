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

// Package aws contains Kubernetes API groups for AWS cloud provider.
package aws

import (
	compute "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	database "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	storage "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, compute.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, database.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, awsv1alpha1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, storage.SchemeBuilder.AddToScheme)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

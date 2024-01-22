// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package pkg contains Kubernetes API groups for Crossplane packages.
package pkg

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	v1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	v1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v1alpha1.SchemeBuilder.AddToScheme,
		v1beta1.SchemeBuilder.AddToScheme,
		v1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

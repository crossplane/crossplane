// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package apis contains Kubernetes API groups
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/apiextensions"
	"github.com/crossplane/crossplane/apis/pkg"
	"github.com/crossplane/crossplane/apis/secrets"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		apiextensions.AddToScheme,
		pkg.AddToScheme,
		secrets.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package controller contains options specific to apiextensions controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.com/crossplane/crossplane/internal/xfn"
)

// Options specific to apiextensions controllers.
type Options struct {
	controller.Options

	// Namespace in which we'll look for image pull secrets for in-cluster
	// private registry authentication when pulling Composition Functions.
	Namespace string

	// ServiceAccount for which we'll find image pull secrets for in-cluster
	// private registry authentication when pulling Composition Functions.
	ServiceAccount string

	// Registry is the default registry to use when pulling containers for
	// Composition Functions
	Registry string

	// FunctionRunner used to run Composition Functions.
	FunctionRunner *xfn.PackagedFunctionRunner
}

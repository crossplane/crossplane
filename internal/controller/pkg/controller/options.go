// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package controller contains options specific to pkg controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.com/crossplane/crossplane/internal/xpkg"
)

// Options specific to pkg controllers.
type Options struct {
	controller.Options

	// Cache for package OCI images.
	Cache xpkg.PackageCache

	// Namespace used to unpack and run packages.
	Namespace string

	// ServiceAccount is the core Crossplane ServiceAccount name.
	ServiceAccount string

	// DefaultRegistry used to pull packages.
	DefaultRegistry string

	// FetcherOptions can be used to add optional parameters to
	// NewK8sFetcher.
	FetcherOptions []xpkg.FetcherOpt

	// PackageRuntime specifies the runtime to use for package runtime.
	PackageRuntime PackageRuntime
}

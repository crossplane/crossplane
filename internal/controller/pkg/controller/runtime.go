// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

// PackageRuntime is the runtime to use for packages with runtime.
type PackageRuntime string

const (
	// PackageRuntimeUnspecified means no package runtime is specified.
	PackageRuntimeUnspecified PackageRuntime = ""

	// PackageRuntimeDeployment uses a Kubernetes Deployment as the package
	// runtime.
	PackageRuntimeDeployment PackageRuntime = "Deployment"

	// PackageRuntimeExternal defer package runtime to an external controller.
	PackageRuntimeExternal PackageRuntime = "External"
)

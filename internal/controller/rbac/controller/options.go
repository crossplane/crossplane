// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package controller contains options specific to rbac controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"
)

// The ManagementPolicy specifies which roles the RBAC manager should manage.
type ManagementPolicy string

const (
	// ManagementPolicyAll indicates that all RBAC manager functionality should
	// be enabled.
	ManagementPolicyAll ManagementPolicy = "All"

	// ManagementPolicyBasic indicates that basic RBAC manager functionality
	// should be enabled. The RBAC manager will create ClusterRoles for each
	// XRD. The ClusterRoles it creates will aggregate to the core Crossplane
	// ClusterRoles (e.g. crossplane, crossplane-admin, etc).
	ManagementPolicyBasic ManagementPolicy = "Basic"
)

// Options specific to rbac controllers.
type Options struct {
	controller.Options

	// ManagementPolicy specifies which roles the RBAC manager should
	// manage.
	ManagementPolicy ManagementPolicy

	// AllowClusterRole is used to determine what additional RBAC
	// permissions may be granted to Providers that request them. The
	// provider may request any permission that appears in the named role.
	AllowClusterRole string

	// DefaultRegistry used by the package manager to pull packages. Must match
	// the package manager's DefaultRegistry in order for the RBAC manager to be
	// able to determine whether two packages are part of the same registry and
	// org.
	DefaultRegistry string
}

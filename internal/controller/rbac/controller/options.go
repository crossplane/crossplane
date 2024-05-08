/*
Copyright 2021 The Crossplane Authors.

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

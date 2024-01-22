// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package rbac implements the controllers of the Crossplane RBAC manager.
package rbac

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/internal/controller/rbac/controller"
	"github.com/crossplane/crossplane/internal/controller/rbac/definition"
	"github.com/crossplane/crossplane/internal/controller/rbac/namespace"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/binding"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/roles"
)

// Setup RBAC manager controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		definition.Setup,
		binding.Setup,
		roles.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}

	if o.ManagementPolicy != controller.ManagementPolicyAll {
		return nil
	}

	return namespace.Setup(mgr, o)
}

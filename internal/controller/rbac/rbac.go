/*
Copyright 2020 The Crossplane Authors.

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

// Package rbac implements the controllers of the Crossplane RBAC manager.
package rbac

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/internal/controller/rbac/definition"
	"github.com/crossplane/crossplane/internal/controller/rbac/namespace"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/binding"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/roles"

	"github.com/crossplane/crossplane/internal/controller/rbac/controller"
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

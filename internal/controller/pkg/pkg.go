// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package pkg implements the controllers of the Crossplane Package manager.
package pkg

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/controller/pkg/manager"
	"github.com/crossplane/crossplane/internal/controller/pkg/resolver"
	"github.com/crossplane/crossplane/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/internal/features"
)

// Setup package controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		manager.SetupConfiguration,
		manager.SetupProvider,
		resolver.Setup,
		revision.SetupConfigurationRevision,
		revision.SetupProviderRevision,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}

	// We only want to start the Function controllers if Functions are enabled.
	if o.Features.Enabled(features.EnableBetaCompositionFunctions) {
		for _, setup := range []func(ctrl.Manager, controller.Options) error{
			manager.SetupFunction,
			revision.SetupFunctionRevision,
		} {
			if err := setup(mgr, o); err != nil {
				return err
			}
		}
	}

	return nil
}

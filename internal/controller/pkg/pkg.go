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

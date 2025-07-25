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

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/controller/pkg/manager"
	"github.com/crossplane/crossplane/internal/controller/pkg/resolver"
	"github.com/crossplane/crossplane/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/internal/controller/pkg/runtime"
	"github.com/crossplane/crossplane/internal/controller/pkg/signature"
	"github.com/crossplane/crossplane/internal/features"
)

// Setup package controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	setupFuncs := []func(ctrl.Manager, controller.Options) error{
		manager.SetupConfiguration,
		manager.SetupProvider,
		manager.SetupFunction,
		resolver.Setup,
		revision.SetupConfigurationRevision,
		revision.SetupProviderRevision,
		revision.SetupFunctionRevision,
	}

	if o.PackageRuntime.For(pkgv1.ProviderKind) == controller.PackageRuntimeDeployment {
		setupFuncs = append(setupFuncs, []func(c ctrl.Manager, options controller.Options) error{
			runtime.SetupProviderRevision,
		}...)
	}

	if o.PackageRuntime.For(pkgv1.FunctionKind) == controller.PackageRuntimeDeployment {
		setupFuncs = append(setupFuncs, []func(c ctrl.Manager, options controller.Options) error{
			runtime.SetupFunctionRevision,
		}...)
	}

	if o.Features.Enabled(features.EnableAlphaSignatureVerification) {
		setupFuncs = append(setupFuncs, []func(c ctrl.Manager, options controller.Options) error{
			signature.SetupProviderRevision,
			signature.SetupConfigurationRevision,
			signature.SetupFunctionRevision,
		}...)
	}

	for _, setup := range setupFuncs {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}

	return nil
}

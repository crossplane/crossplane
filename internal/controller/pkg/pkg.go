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

package pkg

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/internal/controller/pkg/manager"
	"github.com/crossplane/crossplane/internal/controller/pkg/resolver"
	"github.com/crossplane/crossplane/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Setup package controllers.
func Setup(mgr ctrl.Manager, l logging.Logger, c xpkg.Cache, namespace, registry string) error {
	for _, setup := range []func(ctrl.Manager, logging.Logger, string, string) error{
		manager.SetupConfiguration,
		manager.SetupProvider,
	} {
		if err := setup(mgr, l, namespace, registry); err != nil {
			return err
		}
	}
	if err := resolver.Setup(mgr, l, namespace); err != nil {
		return err
	}
	for _, setup := range []func(ctrl.Manager, logging.Logger, xpkg.Cache, string, string) error{
		revision.SetupConfigurationRevision,
		revision.SetupProviderRevision,
	} {
		if err := setup(mgr, l, c, namespace, registry); err != nil {
			return err
		}
	}
	return nil
}

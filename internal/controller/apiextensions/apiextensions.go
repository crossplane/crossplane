// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package apiextensions implements the Crossplane Composition controllers.
package apiextensions

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/internal/controller/apiextensions/composition"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/definition"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/offered"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/usage"
	"github.com/crossplane/crossplane/internal/features"
)

// Setup API extensions controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := composition.Setup(mgr, o); err != nil {
		return err
	}

	if err := definition.Setup(mgr, o); err != nil {
		return err
	}

	if o.Features.Enabled(features.EnableAlphaUsages) {
		if err := usage.Setup(mgr, o); err != nil {
			return err
		}
	}

	return offered.Setup(mgr, o)
}

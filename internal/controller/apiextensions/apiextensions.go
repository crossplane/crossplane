/*
Copyright 2019 The Crossplane Authors.

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

// Package apiextensions implements the Crossplane Composition controllers.
package apiextensions

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/activationpolicy"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composition"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/definition"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/managed"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/offered"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/revision"
)

// Setup API extensions controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := composition.Setup(mgr, o); err != nil {
		return err
	}

	if err := revision.Setup(mgr, o); err != nil {
		return err
	}

	if err := definition.Setup(mgr, o); err != nil {
		return err
	}

	if err := managed.Setup(mgr, o); err != nil {
		return err
	}

	if err := activationpolicy.Setup(mgr, o); err != nil {
		return err
	}

	return offered.Setup(mgr, o)
}

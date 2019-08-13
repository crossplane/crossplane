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

package extensions

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane/pkg/controller/extensions/extension"
	"github.com/crossplaneio/crossplane/pkg/controller/extensions/request"
)

// Controllers passes down config and adds individual controllers to the manager.
type Controllers struct{}

// SetupWithManager adds all GCP controllers to the manager.
func (c *Controllers) SetupWithManager(mgr ctrl.Manager) error {
	if err := (&request.Controller{}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&extension.Controller{}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}

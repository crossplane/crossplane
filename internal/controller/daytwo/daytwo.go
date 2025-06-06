/*
Copyright 2025 The Crossplane Authors.

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

// Package daytwo implements the Crossplane Day Two controllers.
package daytwo

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/internal/controller/daytwo/controller"
	"github.com/crossplane/crossplane/internal/controller/daytwo/operation"
)

// Setup API extensions controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	return operation.Setup(mgr, o)
}

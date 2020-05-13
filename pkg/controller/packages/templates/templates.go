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

package templates

import (
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
)

// SetupStackDefinitions adds a controller that reconciles StackDefinitions.
func SetupStackDefinitions(mgr ctrl.Manager, l logging.Logger) error {
	name := "packages/" + strings.ToLower(v1alpha1.StackDefinitionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.StackDefinition{}).
		Complete(NewStackDefinitionReconciler(mgr.GetClient(), l.WithValues("controller", "stackconfiguration")))
}

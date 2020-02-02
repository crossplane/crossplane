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

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// SetupStackConfigurations adds a controller that reconciles StackConfigurations.
func SetupStackConfigurations(mgr ctrl.Manager, l logging.Logger) error {
	name := "stacks/" + strings.ToLower(v1alpha1.StackConfigurationKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.StackConfiguration{}).
		Complete(NewSetupPhaseReconciler(mgr, l.WithValues("controller", "stackconfiguration")))
}

// SetupStackDefinitions adds a controller that reconciles StackDefinitions.
func SetupStackDefinitions(mgr ctrl.Manager, l logging.Logger) error {
	name := "stacks/" + strings.ToLower(v1alpha1.StackDefinitionKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.StackDefinition{}).
		Complete(NewStackDefinitionReconciler(mgr.GetClient(), l.WithValues("controller", "stackconfiguration")))
}

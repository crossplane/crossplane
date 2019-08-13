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

package deprecateddefaultclass

import (
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// MySQLInstanceController is responsible for adding the default class controller
// for MySQLInstance and its corresponding reconciler to the manager with any runtime configuration.
type MySQLInstanceController struct{}

// SetupWithManager adds a default class controller that reconciles claims
// of kind MySQLInstance to a resource class that declares it as the MySQLInstance
// default.
func (c *MySQLInstanceController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewDeprecatedDefaultClassReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.MySQLInstanceGroupVersionKind),
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.MySQLInstanceKind, controllerBaseName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&databasev1alpha1.MySQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.NoClassReference())).
		WithEventFilter(resource.NewPredicates(resource.NoManagedResourceReference())).
		Complete(r)
}

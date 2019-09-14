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

package defaultclass

import (
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
)

// PostgreSQLInstanceController is responsible for adding the default class controller
// for PostgreSQLInstanceInstance and its corresponding reconciler to the manager with any runtime configuration.
type PostgreSQLInstanceController struct{}

// SetupWithManager adds a default class controller that reconciles claims
// of kind PostgreSQLInstance to a resource class that declares it as the PostgreSQLInstance
// default
func (c *PostgreSQLInstanceController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewDefaultClassReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.PortableClassKind{Singular: databasev1alpha1.PostgreSQLInstanceClassGroupVersionKind, Plural: databasev1alpha1.PostgreSQLInstanceClassListGroupVersionKind},
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.PostgreSQLInstanceKind, controllerBaseName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.HasNoPortableClassReference())).
		WithEventFilter(resource.NewPredicates(resource.HasNoManagedResourceReference())).
		Complete(r)
}

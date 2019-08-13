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

	cachev1alpha1 "github.com/crossplaneio/crossplane/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// RedisClusterController is responsible for adding the default class controller
// for RedisClusterInstance and its corresponding reconciler to the manager with any runtime configuration.
type RedisClusterController struct{}

// SetupWithManager adds a default class controller that reconciles claims
// of kind RedisCluster to a resource class that declares it as the RedisCluster
// default
func (c *RedisClusterController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewDefaultClassReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.PolicyKind{Singular: cachev1alpha1.RedisClusterPolicyGroupVersionKind, Plural: cachev1alpha1.RedisClusterPolicyListGroupVersionKind},
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", cachev1alpha1.RedisClusterKind, controllerBaseName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&cachev1alpha1.RedisCluster{}).
		WithEventFilter(resource.NewPredicates(resource.NoClassReference())).
		WithEventFilter(resource.NewPredicates(resource.NoManagedResourceReference())).
		Complete(r)
}

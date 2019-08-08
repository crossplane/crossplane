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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// AddRedisCluster adds a default class controller that reconciles claims
// of kind RedisCluster to a resource class that declares it as the RedisCluster
// default
func AddRedisCluster(mgr manager.Manager) error {
	r := resource.NewDefaultClassReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.PolicyKind{Singular: cachev1alpha1.RedisClusterPolicyGroupVersionKind, Plural: cachev1alpha1.RedisClusterPolicyListGroupVersionKind},
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", cachev1alpha1.RedisClusterKind, controllerBaseName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create default controller")
	}

	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &cachev1alpha1.RedisCluster{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.NoClassReference()),
		resource.NewPredicates(resource.NoManagedResourceReference()),
	), "cannot watch for %s", cachev1alpha1.RedisClusterGroupVersionKind)
}

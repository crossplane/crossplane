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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// AddBucket adds a default class controller that reconciles claims
// of kind Bucket to a resource class that declares it as the Bucket
// default
func AddBucket(mgr manager.Manager) error {
	r := resource.NewDeprecatedDefaultClassReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.BucketKind, controllerBaseName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create deprecated default controller")
	}

	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &storagev1alpha1.Bucket{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.NoClassReference()),
		resource.NewPredicates(resource.NoManagedResourceReference()),
	), "cannot watch for %s", storagev1alpha1.BucketGroupVersionKind)
}

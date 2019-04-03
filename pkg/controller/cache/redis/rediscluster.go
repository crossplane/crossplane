/*
Copyright 2018 The Crossplane Authors.

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

package redis

import (
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	controllerName = "redisclusters.cache.crossplane.io"
	finalizerName  = "finalizer." + controllerName
)

var log = logging.Logger.WithName("controller." + controllerName)

// Reconciler is the reconciler for RedisCluster objects
type Reconciler struct {
	*corecontroller.Reconciler
}

// AddCluster creates a new RedisCluster Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager
// is Started.
func AddCluster(mgr manager.Manager) error {
	r := &Reconciler{corecontroller.NewReconciler(mgr, controllerName, finalizerName, handlers)}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create controller")
	}
	return errors.Wrap(c.Watch(&source.Kind{Type: &cachev1alpha1.RedisCluster{}}, &handler.EnqueueRequestForObject{}), "cannot watch for RedisClusters")
}

// Reconcile the desired with the actual state of a RedisCluster.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", cachev1alpha1.RedisClusterKindAPIVersion, "request", request)

	c := &cachev1alpha1.RedisCluster{}
	if err := r.Get(ctx, request.NamespacedName, c); err != nil {
		if kerrors.IsNotFound(err) {
			return corecontroller.Result, nil
		}
		return corecontroller.Result, errors.Wrap(err, "cannot get RedisCluster")
	}

	result, err := r.DoReconcile(c)
	return result, errors.Wrapf(err, "cannot reconcile %s/%s", c.GetNamespace(), c.GetName())
}

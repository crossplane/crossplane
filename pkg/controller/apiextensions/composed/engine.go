/*
Copyright 2020 The Crossplane Authors.

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

package composed

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// NewControllerEngine returns a new ControllerEngine instance.
func NewControllerEngine(mgr manager.Manager, log logging.Logger) *ControllerEngine {
	return &ControllerEngine{
		mgr: mgr,
		m:   map[string]chan struct{}{},
		log: log,
	}
}

// ControllerEngine provides tooling for starting and stopping controllers
// in runtime after the manager is started.
type ControllerEngine struct {
	mgr manager.Manager
	m   map[string]chan struct{}

	log logging.Logger
}

// Stop stops the controller that reconciles the given CRD.
func (c *ControllerEngine) Stop(name string) error {
	stop, ok := c.m[name]
	if !ok {
		return nil
	}
	close(stop)
	delete(c.m, name)
	return nil
}

// Start starts an instance controller that will reconcile given CRD.
func (c *ControllerEngine) Start(name string, gvk schema.GroupVersionKind, reconciler reconcile.Reconciler) error {
	stop, exists := c.m[name]
	// TODO(muvaf): when a channel is closed, does it become nil? Find a way
	// to check on the controller to see whether it crashed and needs a restart.
	if exists && stop != nil {
		return nil
	}
	stop = make(chan struct{})
	c.m[name] = stop
	ca, err := cache.New(c.mgr.GetConfig(), cache.Options{
		Scheme: c.mgr.GetScheme(),
		Mapper: c.mgr.GetRESTMapper(),
	})
	if err != nil {
		return err
	}
	go func() {
		<-c.mgr.Leading()
		if err := ca.Start(stop); err != nil {
			c.log.Debug("cannot start controller cache", "controller", name, "error", err)
		}
	}()
	ca.WaitForCacheSync(stop)

	ctrl, err := controller.NewUnmanaged(name, c.mgr,
		controller.Options{
			Reconciler: reconciler,
		})
	if err != nil {
		return errors.Wrap(err, "cannot create an unmanaged controller")
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	if err := ctrl.Watch(source.NewKindWithCache(u, ca), &handler.EnqueueRequestForObject{}); err != nil {
		return errors.Wrap(err, "cannot set watch parameters on controller")
	}

	go func() {
		<-c.mgr.Leading()
		// TODO(muvaf): Handle the case where controller crashes. Do we need to
		// detect and restart?
		c.log.Info("composite controller", "starting the controller", name)
		if err := ctrl.Start(stop); err != nil {
			c.log.Debug("composite controller", "cannot start controller", name, "error", err)
		}
		c.log.Info("composite controller", "controller has been stopped", name)
	}()

	return nil
}

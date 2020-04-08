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
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1alpha12 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1/instance"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute
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
func (c *ControllerEngine) Start(name string, gvk schema.GroupVersionKind) error {
	stop, exists := c.m[name]
	// todo(muvaf): when a channel is closed, does it become nil? Find a way
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
			Reconciler: newInstanceReconciler(name, c.mgr, gvk, c.log),
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
		// todo: handle the case where controller crashes since we deleted the CRD
		c.log.Info("instance controller", "starting the controller", name)
		if err := ctrl.Start(stop); err != nil {
			c.log.Debug("instance controller", "cannot start controller", name, "error", err)
		}
		c.log.Info("instance controller", "controller has been stopped", name)
	}()

	return nil
}

// newInstanceReconciler returns a new *instanceReconciler.
func newInstanceReconciler(name string, mgr manager.Manager, gvk schema.GroupVersionKind, log logging.Logger) *instanceReconciler {
	ni := func() *instance.InfraInstance {
		cr := &instance.InfraInstance{}
		cr.SetGroupVersionKind(gvk)
		return cr
	}
	cl := NewClientForUnregistered(mgr.GetClient(), mgr.GetScheme(), runtime.DefaultUnstructuredConverter)
	kube := resource.ClientApplicator{
		Client:     cl,
		Applicator: resource.NewAPIPatchingApplicator(cl),
	}
	return &instanceReconciler{
		client:      kube,
		newInstance: ni,
		composition: defaultCRComposition(kube),
		log:         log,
		record:      event.NewAPIRecorder(mgr.GetEventRecorderFor(name)),
	}
}

func defaultCRComposition(kube client.Client) crComposition {
	return crComposition{
		SpecOps: &SelectorResolver{client: kube},
	}
}

type crComposition struct {
	SpecOps
}

// instanceReconciler reconciles the generic CRD that is generated via InfrastructureDefinition.
type instanceReconciler struct {
	client      resource.ClientApplicator // todo: only target reconciler needs this. use client.Client when you separate the two
	newInstance func() *instance.InfraInstance
	composition crComposition
	//targets     crTargetResources

	log    logging.Logger
	record event.Recorder
}

// Reconcile reconciles given custom resource.
func (r *instanceReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cr := r.newInstance()
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get instance")
	}

	if err := r.composition.ResolveSelector(ctx, cr); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot resolve composition selector")
	}

	comp := &v1alpha1.Composition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: cr.Spec.CompositionReference.Name}, comp); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot fetch the composition")
	}

	for i, target := range comp.Spec.To {
		if _, err := r.ReconcileTargetResource(ctx, cr, target, i); err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot reconcile one of the target resources")
		}
	}
	isReady := true
	for _, targetRef := range cr.Spec.ResourceReferences {
		c, err := r.CheckTargetResource(ctx, targetRef)
		if err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot check one of the target resources")
		}
		if c.Status != v1.ConditionTrue {
			isReady = false
			break
		}
	}
	if isReady {
		cr.Status.SetConditions(v1alpha12.Available())
		return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), "cannot update the status of the instance")
	}
	return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), "cannot update the status of the instance")

	// generate actual CR
	//   overlay patches to the base
	// check relative index on spec ref. if it exists and same kind, add metadata. (array will have same order as composition)
	// apply generated CR -> this will also retrieve it from api-server.
	// write ref to instance spec if relative index is empty
	// fetch the secret of CR, choose the given keys and return the values in a map.

	// main reconciler will publish the keys to the instance secret.
}

func (r *instanceReconciler) ReconcileTargetResource(ctx context.Context, cr *instance.InfraInstance, target v1alpha1.TargetResource, index int) (managed.ConnectionDetails, error) {
	result := target.Base.DeepCopy()
	from, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert composition instance to unstructured object")
	}
	fromPaved := fieldpath.Pave(from)
	for i, patch := range target.Patches {
		// todo: figure out a better interface for this Patch function.
		if err := patch.Patch(fromPaved, result); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("cannot apply the patch at index %d on result", i))
		}
	}
	// TODO(muvaf): assumes that cr is cluster-scoped.
	result.SetGenerateName(fmt.Sprintf("%s-", cr.GetName()))
	if len(cr.Spec.ResourceReferences) > index {
		if cr.Spec.ResourceReferences[index].GroupVersionKind() != result.GroupVersionKind() {
			return nil, errors.New(fmt.Sprintf("resource reference at index %d is not of the same kind with the resource in composition at the same index", index))
		}
		result.SetName(cr.Spec.ResourceReferences[index].Name)
		result.SetNamespace(cr.Spec.ResourceReferences[index].Name)
	}
	if err := r.client.Apply(ctx, result, resource.MustBeControllableBy(cr.GetUID())); err != nil {
		return nil, errors.Wrap(err, "cannot apply the target resource")
	}
	// todo: assumes that this function is called in the right order.
	if len(cr.Spec.ResourceReferences) < index {
		cr.Spec.ResourceReferences = append(cr.Spec.ResourceReferences, *meta.ReferenceTo(result, result.GroupVersionKind()))
	}
	// todo TODO NOW: HANDLE SECRETS
	return nil, nil
}

func (r *instanceReconciler) CheckTargetResource(ctx context.Context, ref v1.ObjectReference) (v1alpha12.Condition, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ref.GroupVersionKind())
	// NamespacedNameOf should accept value as it doesn't manipulate the input
	if err := r.client.Get(ctx, meta.NamespacedNameOf(&ref), u); err != nil {
		return v1alpha12.Condition{}, err
	}
	c, err := GetCondition(u, v1alpha12.TypeReady)
	return c, errors.Wrap(err, "cannot fetch the conditions from target resource")
}

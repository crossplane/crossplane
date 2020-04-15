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
	"strings"
	"time"

	v1alpha12 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	ni := func() instance.CompositionInstance {
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

// SpecOps lists the operations that are done on the spec of instance.
type SpecOps interface {
	ResolveSelector(ctx context.Context, cr instance.CompositionInstance) error
}

type crComposition struct {
	SpecOps
}

type TargetReconciler interface {
	Apply(context.Context, v1.ObjectReference, v1alpha1.TargetResource) (v1.ObjectReference, error)
	GetConnectionDetails(context.Context, v1.ObjectReference, v1alpha1.TargetResource) (managed.ConnectionDetails, error)
}

type APITargetReconciler struct {
	client   resource.ClientApplicator
	instance instance.CompositionInstance
}

func (r *APITargetReconciler) Apply(ctx context.Context, ref v1.ObjectReference, target v1alpha1.TargetResource) (v1.ObjectReference, error) {
	result := target.Base.DeepCopy()
	paved := fieldpath.Pave(r.instance.UnstructuredContent())
	for i, patch := range target.Patches {
		if err := patch.Patch(paved, result); err != nil {
			return v1.ObjectReference{}, errors.Wrap(err, fmt.Sprintf("cannot apply the patch at index %d on result", i))
		}
	}
	result.SetGenerateName(fmt.Sprintf("%s-", r.instance.GetName()))
	result.SetName(ref.Name)
	result.SetNamespace(ref.Namespace)
	if err := r.client.Apply(ctx, result, resource.MustBeControllableBy(r.instance.GetUID())); err != nil {
		return v1.ObjectReference{}, errors.Wrap(err, "cannot apply the target resource")
	}
	return *meta.ReferenceTo(result, result.GroupVersionKind()), nil
}

func (r *APITargetReconciler) GetConnectionDetails(ctx context.Context, ref v1.ObjectReference, target v1alpha1.TargetResource) (managed.ConnectionDetails, error) {
	u := &unstructured.Unstructured{}
	if err := r.client.Get(ctx, meta.NamespacedNameOf(&ref), u); err != nil {
		return nil, err
	}
	// TODO(muvaf): An unstructured.Unstructured based ManagedInstance struct
	// similar to InfraInstance would make these things easier.
	secretRefObj, exists, err := unstructured.NestedMap(u.UnstructuredContent(), strings.Split("spec.writeConnectionSecretToRef", ".")...)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	secretRef := &v1.ObjectReference{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(secretRefObj, secretRef); err != nil {
		return nil, err
	}
	secret := &v1.Secret{}
	if err := r.client.Get(ctx, meta.NamespacedNameOf(secretRef), secret); err != nil {
		return nil, err
	}
	out := managed.ConnectionDetails{}
	for _, pair := range target.ConnectionDetails {
		key := pair.FromConnectionSecretKey
		if pair.Name != nil {
			key = *pair.Name
		}
		out[key] = secret.Data[pair.FromConnectionSecretKey]
	}
	return out, nil
}

// instanceReconciler reconciles the generic CRD that is generated via InfrastructureDefinition.
type instanceReconciler struct {
	client      resource.ClientApplicator // todo: only target reconciler needs this. use client.Client when you separate the two
	newInstance func() instance.CompositionInstance
	composition crComposition

	log    logging.Logger
	record event.Recorder
}

// Reconcile reconciles given custom resource.
func (r *instanceReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
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
	if err := r.client.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot fetch the composition")
	}
	tr := &APITargetReconciler{
		client:   r.client,
		instance: cr,
	}
	refs := cr.GetResourceReferences()
	for i, target := range comp.Spec.To {
		ref := v1.ObjectReference{}
		newRef := v1.ObjectReference{}
		var err error
		if len(refs) > i {
			ref = refs[i]
		}
		newRef, err = tr.Apply(ctx, ref, target)
		if err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot reconcile one of the target resources")
		}
		if len(refs) <= i {
			refs = append(refs, newRef)
		} else {
			refs[i] = newRef
		}
		cr.SetResourceReferences(refs)
		if err := r.client.Update(ctx, cr); err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot update instance cr")
		}
	}
	cr.SetConditions(v1alpha12.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), "cannot update status of instance cr")

	//conn := managed.ConnectionDetails{}
	//for i, ref := range cr.GetResourceReferences() {
	//	out, err := tr.GetConnectionDetails(ctx, ref, comp.Spec.To[i])
	//	if err != nil {
	//		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot get connection details")
	//	}
	//	for key, val := range out {
	//		conn[key] = val
	//	}
	//}

	// generate actual CR
	//   overlay patches to the base
	// check relative index on spec ref. if it exists and same kind, add metadata. (array will have same order as composition)
	// apply generated CR -> this will also retrieve it from api-server.
	// write ref to instance spec if relative index is empty
	// fetch the secret of CR, choose the given keys and return the values in a map.

	// main reconciler will publish the keys to the instance secret.
}

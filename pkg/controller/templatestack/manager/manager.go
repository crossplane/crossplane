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

package manager

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

const (
	controllerName   = "templatestackmanager.stacks.crossplane.io"
	reconcileTimeout = 1 * time.Minute
	// requeueOnWait    = 30 * time.Second
)

var log = logging.Logger.WithName("controller." + controllerName)

// A Reconciler reconciles Unstructured objects.
type Reconciler struct {
	client client.Client
	stack  types.NamespacedName
}

// Controller is responsible for adding the Unstructured
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct {
	Stack types.NamespacedName
}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	client := mgr.GetClient()

	r := &Reconciler{
		client: client,
		stack:  c.Stack,
	}

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	findstack := &unstructured.Unstructured{}
	gvk := v1alpha1.StackGroupVersionKind
	findstack.SetGroupVersionKind(gvk)
	findstack.SetName(c.Stack.Name)
	findstack.SetNamespace(c.Stack.Namespace)

	if err := client.Get(ctx, c.Stack, findstack); err != nil {
		return err
	}

	stack := &v1alpha1.Stack{}
	if data, err := findstack.MarshalJSON(); err != nil {
		return err
	} else if err := json.Unmarshal(data, stack); err != nil {
		return err

	}

	for _, crd := range stack.Spec.CRDs {
		managedType := &unstructured.Unstructured{}
		managedType.SetAPIVersion(crd.APIVersion)
		managedType.SetKind(crd.Kind)

		err := ctrl.NewControllerManagedBy(mgr).
			Named(controllerName).
			For(managedType).
			Complete(r)

		if err != nil {
			return err
		}
	}
	return nil
}

// Reconcile scheduled Unstructured objects by applying their templates
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// Get the Stack responsible for this TSM
	stack := v1alpha1.Stack{}
	if err := r.client.Get(ctx, r.stack, &stack); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}

		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", r.stack.Namespace, r.stack.Name)
	}

	// Get the resource TSM is managing
	app := &unstructured.Unstructured{}
	if err := r.client.Get(ctx, req.NamespacedName, app); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", req.NamespacedName.Namespace, req.NamespacedName.Name)
	}

	// Each template must be valid yaml. APIVersion, Kind, and Name must
	// be available so the correct object can be retrieved/compared
	// TODO(displague) design doc should not leave this to chance
	// perhaps the first 4 lines are required to provide this.

	templates := getTemplates(stack, app.GroupVersionKind())

	syncer := templateSyncer{client: r.client, templates: templates}
	updated, errs := syncer.Sync(ctx)
	_, _ = updated, errs

	// TODO(displague) determine when we use a statusWriter or Patch stack
	// TODO(displague) include errors in the map of variables presented
	// TODO(displague) apply template variables
	// pseudocode:
	// newstatus = applytemplates(stack.template.status, updated)
	// statusWriter:=r.client.Status()
	// statusWriter.Patch(ctx, stack, client.MergeFrom(stack))

	return reconcile.Result{Requeue: true}, errors.Wrapf(r.client.Update(ctx, app), "cannot update %s %s", req.Namespace, req.Name)

}

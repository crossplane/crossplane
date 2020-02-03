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

package templates

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// StackDefinitionReconciler copies a StackDefinition over to a Stack.
// The idea is that later, we will reassess the data model and try to unify Stacks and
// StackDefinitions. At that time, the Stack controller logic will likely just be pointed
// over at StackDefinitions.
type StackDefinitionReconciler struct {
	Client client.Client
	Log    logr.Logger
}

const (
	stackDefinitionTimeout = 60 * time.Second
)

// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations/status,verbs=get;update;patch

// Reconcile watches for stack configurations and configures render phase controllers in response to a stack configuration
func (r *StackDefinitionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stackDefinitionTimeout)
	defer cancel()

	i := &v1alpha1.StackDefinition{}
	if err := r.Client.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			r.Log.V(logging.Debug).Info("Requested stack definition not found; ignoring", "request", req)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Error fetching stack definition", "request", req, "stackDefinition", i)
		return ctrl.Result{}, err
	}

	s := &v1alpha1.Stack{}
	if err := r.Client.Get(ctx, req.NamespacedName, s); err != nil {
		if kerrors.IsNotFound(err) {
			s.SetNamespace(req.Namespace)
			s.SetName(req.Name)
			r.Log.V(logging.Debug).Info("Stack not found; creating from stack definition", "request", req, "stackDefinition", i)
			return ctrl.Result{}, r.createStack(ctx, i, s)
		}

		r.Log.Error(err, "Error fetching stack", "request", req, "stackDefinition", i)
		return ctrl.Result{}, err
	}

	r.Log.V(logging.Debug).Info("Stack exists; updating from stack definition", "request", req, "stackDefinition", i, "stack", s)
	return ctrl.Result{}, r.syncStack(ctx, i, s)
}

func (r *StackDefinitionReconciler) createStack(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Stack) error {
	deepCopyStackDefinitionToStack(sd, s)
	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(sd, v1alpha1.StackDefinitionGroupVersionKind),
	})
	return r.Client.Create(ctx, s)
}

func (r *StackDefinitionReconciler) syncStack(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Stack) error {
	deepCopyStackDefinitionToStack(sd, s)
	return r.Client.Update(ctx, s)
}

func deepCopyStackDefinitionToStack(sd *v1alpha1.StackDefinition, s *v1alpha1.Stack) {
	s.Spec.AppMetadataSpec = *sd.Spec.AppMetadataSpec.DeepCopy()
	s.Spec.CRDs = sd.Spec.CRDs.DeepCopy()
	s.Spec.Controller = *sd.Spec.Controller.DeepCopy()
	s.Spec.Permissions = *sd.Spec.Permissions.DeepCopy()
	s.ObjectMeta.SetLabels(sd.ObjectMeta.GetLabels())
}

// SetupWithManager is a convenience method to register the reconciler with a controller manager.
func (r *StackDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.StackDefinition{}).
		Complete(r)
}

// NewStackDefinitionReconciler creates a stack definition reconciler and initializes all of its fields.
// It mostly exists to make it easier to create a reconciler and check its initialization result at the same time.
func NewStackDefinitionReconciler(c client.Client, l logr.Logger) *StackDefinitionReconciler {
	return &StackDefinitionReconciler{
		Client: c,
		Log:    l,
	}
}

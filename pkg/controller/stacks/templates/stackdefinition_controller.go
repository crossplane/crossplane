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

	rbacv1 "k8s.io/api/rbac/v1"
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
	Log    logging.Logger
}

const (
	stackDefinitionTimeout = 60 * time.Second
)

// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations/status,verbs=get;update;patch

// Reconcile watches for stackdefinition and creates a Stack in response
func (r *StackDefinitionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stackDefinitionTimeout)
	defer cancel()

	i := &v1alpha1.StackDefinition{}
	if err := r.Client.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			r.Log.Debug("Requested stack definition not found; ignoring", "request", req)
			return ctrl.Result{}, nil
		}
		r.Log.Debug("Error fetching stack definition", "request", req, "stackDefinition", i, "error", err)
		return ctrl.Result{}, err
	}

	s := &v1alpha1.Stack{}
	if err := r.Client.Get(ctx, req.NamespacedName, s); err != nil {
		if kerrors.IsNotFound(err) {
			s.SetNamespace(req.Namespace)
			s.SetName(req.Name)
			r.Log.Debug("Stack not found; creating from stack definition", "request", req, "stackDefinition", i)
			if err = r.createStack(ctx, i, s); err != nil && !kerrors.IsAlreadyExists(err) {
				r.Log.Debug("Error creating a Stack from StackDefinition", "request", req, "stackDefinition", i, "error", err)
				return ctrl.Result{}, err
			}
		}

		r.Log.Debug("Error fetching stack", "request", req, "stackDefinition", i, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Debug("Stack exists; updating from stack definition", "request", req, "stackDefinition", i, "stack", s)
	return ctrl.Result{}, r.syncStack(ctx, i, s)
}

func (r *StackDefinitionReconciler) createStack(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Stack) error {
	sd.DeepCopyIntoStack(s)
	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(sd, v1alpha1.StackDefinitionGroupVersionKind),
	})

	rule := rbacv1.PolicyRule{
		Verbs:         []string{"get", "list", "watch"},
		APIGroups:     []string{"stacks.crossplane.io"},
		Resources:     []string{"stackdefinitions", "stackdefinitions/status"},
		ResourceNames: []string{sd.GetName()},
	}
	s.Spec.Permissions.Rules = append(s.Spec.Permissions.Rules, rule)
	return r.Client.Create(ctx, s)
}

func (r *StackDefinitionReconciler) syncStack(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Stack) error {
	sCopy := s.DeepCopy()
	sd.DeepCopyIntoStack(sCopy)
	return r.Client.Patch(ctx, s, client.MergeFrom(sCopy))
}

// NewStackDefinitionReconciler creates a stack definition reconciler and initializes all of its fields.
// It mostly exists to make it easier to create a reconciler and check its initialization result at the same time.
func NewStackDefinitionReconciler(c client.Client, l logging.Logger) *StackDefinitionReconciler {
	return &StackDefinitionReconciler{
		Client: c,
		Log:    l,
	}
}

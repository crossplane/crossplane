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

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
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

	longWait = 60 * time.Second
)

// +kubebuilder:rbac:groups=packages.crossplane.io,resources=stackconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=packages.crossplane.io,resources=stackconfigurations/status,verbs=get;update;patch

// Reconcile watches for stackdefinition and creates a Package in response
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

	s := &v1alpha1.Package{}
	err := r.Client.Get(ctx, req.NamespacedName, s)
	if kerrors.IsNotFound(err) {
		s.SetNamespace(req.Namespace)
		s.SetName(req.Name)
		r.Log.Debug("Package not found; creating from stack definition", "request", req, "stackDefinition", i)
		if err := r.createPackage(ctx, i, s); err != nil && !kerrors.IsAlreadyExists(err) {
			r.Log.Debug("Error creating a Package from StackDefinition", "request", req, "stackDefinition", i, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: longWait}, nil
	}
	if err != nil && !kerrors.IsNotFound(err) {
		r.Log.Debug("Error fetching package", "request", req, "stackDefinition", i, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Debug("Package exists; updating from stack definition", "request", req, "stackDefinition", i, "package", s)
	return ctrl.Result{RequeueAfter: longWait}, r.syncPackage(ctx, i, s)
}

func (r *StackDefinitionReconciler) createPackage(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Package) error {
	sd.DeepCopyIntoPackage(s)
	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(sd, v1alpha1.StackDefinitionGroupVersionKind),
	})

	rule := rbacv1.PolicyRule{
		Verbs:         []string{"get", "list", "watch"},
		APIGroups:     []string{"packages.crossplane.io"},
		Resources:     []string{"stackdefinitions", "stackdefinitions/status"},
		ResourceNames: []string{sd.GetName()},
	}
	s.Spec.Permissions.Rules = append(s.Spec.Permissions.Rules, rule)
	return r.Client.Create(ctx, s)
}

func (r *StackDefinitionReconciler) syncPackage(ctx context.Context, sd *v1alpha1.StackDefinition, s *v1alpha1.Package) error {
	sCopy := s.DeepCopy()
	sd.DeepCopyIntoPackage(sCopy)
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

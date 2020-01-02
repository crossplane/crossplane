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

package target

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	workloadv1alpha1 "github.com/crossplaneio/crossplane/apis/workload/v1alpha1"
)

const (
	controllerName   = "autotarget.workload.crossplane.io"
	reconcileTimeout = 1 * time.Minute

	errGetKubernetesCluster = "unable to get KubernetesCluster"
	errCreateOrUpdateTarget = "unable to create or update KubernetesTarget"
	errTargetConflict       = "cannot establish control of existing KubernetesTarget"
)

var log = logging.Logger.WithName("controller." + controllerName)

func clusterIsBound(obj runtime.Object) bool {
	r, ok := obj.(*computev1alpha1.KubernetesCluster)
	if !ok {
		return false
	}

	return r.GetBindingPhase() == runtimev1alpha1.BindingPhaseBound
}

// Controller is responsible for adding the KubernetesTarget auto-creation
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		kube: mgr.GetClient(),
	}

	p := resource.NewPredicates(clusterIsBound)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&computev1alpha1.KubernetesCluster{}).
		WithEventFilter(p).
		Complete(r)
}

// A Reconciler creates KubernetesTargets for KubernetesClusters.
type Reconciler struct {
	kube client.Client
}

// Reconcile attempts to create a KubernetesTarget for a KubernetesCluster.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", workloadv1alpha1.KubernetesTargetKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	cluster := &computev1alpha1.KubernetesCluster{}
	if err := r.kube.Get(ctx, req.NamespacedName, cluster); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrap(err, errGetKubernetesCluster)
	}

	// This KubernetesCluster has been deleted. The KubernetesTarget will be
	// cleaned up by garbage collection.
	if meta.WasDeleted(cluster) {
		return reconcile.Result{Requeue: false}, nil
	}

	target := &workloadv1alpha1.KubernetesTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-target", cluster.GetUID()),
			Namespace:       req.Namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.ReferenceTo(cluster, computev1alpha1.KubernetesClusterGroupVersionKind))},
		},
		Spec: workloadv1alpha1.KubernetesTargetSpec{
			ConnectionSecretRef: cluster.GetWriteConnectionSecretToReference(),
		},
	}

	_, err := util.CreateOrUpdate(ctx, r.kube, target, func() error {
		if c := metav1.GetControllerOf(target); c == nil || c.UID != cluster.GetUID() {
			return errors.New(errTargetConflict)
		}

		target.Spec.ConnectionSecretRef = cluster.GetWriteConnectionSecretToReference()

		return nil
	})

	return reconcile.Result{}, errors.Wrap(err, errCreateOrUpdateTarget)
}

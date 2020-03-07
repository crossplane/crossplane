/*

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

package traitdefinition

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

// TraitDefinitionReconciler reconciles a TraitDefinition object
type TraitDefinitionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.oam.dev,resources=traitdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.oam.dev,resources=traitdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete

func (r *TraitDefinitionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("traitDefinition definition", req.NamespacedName)

	log.Info("reconciling trait definition")

	var traitDefinition corev1alpha2.TraitDefinition
	if err := r.Get(ctx, req.NamespacedName, &traitDefinition); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Get a traitDefinition", "definition ref", traitDefinition.Spec.Reference.Name)

	var traitCRD extv1.CustomResourceDefinition
	crdName := client.ObjectKey{Name: traitDefinition.Spec.Reference.Name, Namespace: ""}
	if err := r.Get(ctx, crdName, &traitCRD); err != nil {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Minute,
		}, err
	}
	log.Info("Get the corresponding CRD", "CRD Name", traitCRD.Name, "Owner", traitCRD.OwnerReferences)
	// merge with
	falseVar := false
	newCRD := traitCRD.DeepCopy()
	newCRD.ObjectMeta.ManagedFields = nil
	newCRD.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: traitDefinition.APIVersion,
			Kind:       traitDefinition.Kind,
			Name:       traitDefinition.Name,
			UID:        traitDefinition.UID,
			Controller: &falseVar,
		},
	}
	traitCRD.Annotations["lastModifiedTime"] = time.Now().String()
	if err := r.Patch(ctx, newCRD, client.MergeFrom(&traitCRD)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TraitDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha2.TraitDefinition{}).
		Complete(r)
}

/*
Copyright 2025 The Crossplane Authors.

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

// Package customresourcesgate implements a CustomResourceReconciler to report GKVs status to a Gate.
// This reconciler requires cluster scoped GET,LIST,WATCH on customresourcedefinitions[apiextensions.k8s.io]
package customresourcesgate

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// Reconciler reconciles a CustomResourceDefinitions in order to gate and wait
// on CRD readiness to start downstream controllers.
type Reconciler struct {
	log  logging.Logger
	gate controller.Gate
}

// Reconcile reconciles CustomResourceDefinitions and reports ready and unready GVKs to the gate.
func (r *Reconciler) Reconcile(_ context.Context, crd *apiextensionsv1.CustomResourceDefinition) (ctrl.Result, error) {
	established := isEstablished(crd)
	gkvs := toGVKs(crd)

	switch {
	// CRD is not ready or being deleted.
	case !established || !crd.GetDeletionTimestamp().IsZero():
		for gvk := range gkvs {
			r.log.Debug("gvk is not ready", "gvk", gvk)
			r.gate.Set(gvk, false)
		}

		return ctrl.Result{}, nil

	// CRD is ready.
	default:
		for gvk, served := range gkvs {
			if served {
				r.log.Debug("gvk is ready", "gvk", gvk)
				r.gate.Set(gvk, true)
			}
		}
	}

	return ctrl.Result{}, nil
}

func toGVKs(crd *apiextensionsv1.CustomResourceDefinition) map[schema.GroupVersionKind]bool {
	gvks := make(map[schema.GroupVersionKind]bool, len(crd.Spec.Versions))
	for _, version := range crd.Spec.Versions {
		gvks[schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}] = version.Served
	}

	return gvks
}

func isEstablished(crd *apiextensionsv1.CustomResourceDefinition) bool {
	if len(crd.Status.Conditions) > 0 {
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established {
				return cond.Status == apiextensionsv1.ConditionTrue
			}
		}
	}

	return false
}

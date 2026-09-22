package runtime

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
)

// EnqueuePackageRevisionsForRuntimeConfig enqueues a reconcile for all package
// revisions that use a ControllerConfig or DeploymentRuntimeConfig.
func EnqueuePackageRevisionsForRuntimeConfig(kube client.Client, l v1.PackageRevisionList, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		rc, ok := o.(*v1beta1.DeploymentRuntimeConfig)
		if !ok {
			return nil
		}

		rl := l.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Guaranteed to be PackageRevisionList.
		if err := kube.List(ctx, rl); err != nil {
			log.Debug("Cannot list package revisions while attempting to enqueue from runtime config", "error", err)
			return nil
		}

		var matches []reconcile.Request

		for _, rev := range rl.GetRevisions() {
			rt, ok := rev.(v1.PackageRevisionWithRuntime)
			if !ok {
				continue
			}

			ref := rt.GetRuntimeConfigRef()
			if ref != nil && ref.Name == rc.GetName() {
				matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.GetName()}})
			}
		}

		return matches
	})
}

// EnqueueProviderRevisionsForMRDs enqueues a reconcile for the provider
// revision that controls a ManagedResourceDefinition when that MRD is active.
func EnqueueProviderRevisionsForMRDs(log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		mrd, ok := o.(*extv1alpha1.ManagedResourceDefinition)
		if !ok {
			return nil
		}

		if !mrd.Spec.State.IsActive() {
			return nil
		}

		owner := metav1.GetControllerOf(mrd)
		if owner == nil || owner.Kind != v1.ProviderRevisionKind {
			return nil
		}

		if gv, err := schema.ParseGroupVersion(owner.APIVersion); err != nil || gv.Group != v1.Group {
			return nil
		}

		log.Debug("Enqueuing provider revision for activated managed resource definition", "mrd", mrd.GetName(), "provider-revision", owner.Name)

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: owner.Name}}}
	})
}

// mrdActivated returns a predicate that only lets events for active
// ManagedResourceDefinitions through. Update events additionally require the
// state to have changed, to avoid enqueuing reconciles for MRD status churn.
// Create events for already active MRDs matter to replay activations after an
// informer restart.
func mrdActivated() predicate.Funcs {
	isActive := func(o client.Object) bool {
		mrd, ok := o.(*extv1alpha1.ManagedResourceDefinition)
		return ok && mrd.Spec.State.IsActive()
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isActive(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !isActive(e.ObjectOld) && isActive(e.ObjectNew)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

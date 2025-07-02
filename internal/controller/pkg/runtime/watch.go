package runtime

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
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

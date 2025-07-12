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

package operation

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// EnqueueOperationsForFunctionRevision enqueues a reconcile for all Operations
// that reference a Function when a FunctionRevision changes.
func EnqueueOperationsForFunctionRevision(kube client.Reader, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		fr, ok := o.(*pkgv1.FunctionRevision)
		if !ok {
			return nil
		}

		// Get the function name from the revision's parent package label
		name := fr.GetLabels()[pkgv1.LabelParentPackage]
		if name == "" {
			log.Debug("FunctionRevision has no parent package label", "revision", fr.GetName())
			return nil
		}

		// List all Operations to find those that reference this function
		ops := &v1alpha1.OperationList{}
		if err := kube.List(ctx, ops); err != nil {
			log.Debug("Cannot list Operations while attempting to enqueue from FunctionRevision", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, op := range ops.Items {
			for _, fn := range op.Spec.Pipeline {
				if fn.FunctionRef.Name != name {
					continue
				}

				log.Debug("Enqueuing Operation for FunctionRevision change",
					"operation", op.GetName(),
					"function-revision", fr.GetName(),
					"function", name)
				matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: op.GetNamespace(),
					Name:      op.GetName(),
				}})

				// Functions are unique within the pipeline.
				break
			}
		}

		return matches
	})
}

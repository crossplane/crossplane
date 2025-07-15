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

package revision

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// EnqueueCompositionRevisionsForFunctionRevision enqueues a reconcile for all CompositionRevisions
// that reference a Function when a FunctionRevision changes.
func EnqueueCompositionRevisionsForFunctionRevision(kube client.Reader, log logging.Logger) handler.EventHandler {
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

		// List all CompositionRevisions to find those that reference this function
		revs := &v1.CompositionRevisionList{}
		if err := kube.List(ctx, revs); err != nil {
			log.Debug("Cannot list CompositionRevisions while attempting to enqueue from FunctionRevision", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, rev := range revs.Items {
			for _, fn := range rev.Spec.Pipeline {
				if fn.FunctionRef.Name != name {
					continue
				}

				log.Debug("Enqueuing CompositionRevision for FunctionRevision change",
					"composition-revision", rev.GetName(),
					"function-revision", fr.GetName(),
					"function", name)
				matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.GetName()}})

				// Functions are unique within the pipeline.
				break
			}
		}

		return matches
	})
}

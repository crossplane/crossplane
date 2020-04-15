/*
Copyright 2020 The Crossplane Authors.

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

package composed

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1/instance"
)

// SelectorResolver is used to resolve the composition selector on the instance
// to composition reference.
type SelectorResolver struct {
	// todo: it has to be the unregistered client actually.

	client client.Client
}

// ResolveSelector resolves selector to a reference if it doesn't exist.
func (r *SelectorResolver) ResolveSelector(ctx context.Context, cr instance.CompositionInstance) error {
	if cr.GetCompositionReference() != nil {
		return nil
	}
	sel := cr.GetCompositionSelector()
	if sel == nil {
		return errors.New("no composition selector to resolve")
	}
	list := &v1alpha1.CompositionList{}
	if err := r.client.List(ctx, list, client.MatchingLabels(sel.MatchLabels)); err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return errors.New("no composition has been found that has the given labels")
	}
	// TODO(muvaf): need to block the deletion of composition via finalizer once it's selected since it's integral to this resource.
	// TODO(muvaf): We don't rely on UID in practice. It should not be there because it will make confusion if the resource is backed up and restored to another cluster
	cr.SetCompositionReference(meta.ReferenceTo(&list.Items[0], v1alpha1.CompositionGroupVersionKind))
	return r.client.Update(ctx, cr)
}

/*
Copyright 2023 The Crossplane Authors.

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

package usage

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errUpdateAfterResolveSelector            = "cannot update usage after resolving selector"
	errResolveSelectorForUsingResource       = "cannot resolve selector at \"spec.by.resourceSelector\""
	errResolveSelectorForUsedResource        = "cannot resolve selector at \"spec.of.resourceSelector\""
	errNoSelectorToResolve                   = "no selector defined for resolving"
	errListResourceMatchingLabels            = "cannot list resources matching labels"
	errFmtResourcesNotFound                  = "no %q found matching labels: %q"
	errFmtResourcesNotFoundWithControllerRef = "no %q found matching labels: %q and with same controller reference"
)

type apiSelectorResolver struct {
	client client.Client
}

func newAPISelectorResolver(c client.Client) *apiSelectorResolver {
	return &apiSelectorResolver{client: c}
}

func (r *apiSelectorResolver) resolveSelectors(ctx context.Context, u *v1alpha1.Usage) error {
	of := u.Spec.Of
	by := u.Spec.By

	if of.ResourceRef == nil || len(of.ResourceRef.Name) == 0 {
		if err := r.resolveSelector(ctx, u, &of); err != nil {
			return errors.Wrap(err, errResolveSelectorForUsedResource)
		}
		u.Spec.Of = of
		if err := r.client.Update(ctx, u); err != nil {
			return errors.Wrap(err, errUpdateAfterResolveSelector)
		}
	}

	if by == nil {
		return nil
	}

	if by.ResourceRef == nil || len(by.ResourceRef.Name) == 0 {
		if err := r.resolveSelector(ctx, u, by); err != nil {
			return errors.Wrap(err, errResolveSelectorForUsingResource)
		}
		u.Spec.By = by
		if err := r.client.Update(ctx, u); err != nil {
			return errors.Wrap(err, errUpdateAfterResolveSelector)
		}
	}

	return nil
}

func (r *apiSelectorResolver) resolveSelector(ctx context.Context, u *v1alpha1.Usage, rs *v1alpha1.Resource) error {
	l := composed.NewList(composed.FromReferenceToList(v1.ObjectReference{
		APIVersion: rs.APIVersion,
		Kind:       rs.Kind,
	}))

	if rs.ResourceSelector == nil {
		return errors.New(errNoSelectorToResolve)
	}
	if err := r.client.List(ctx, l, client.MatchingLabels(rs.ResourceSelector.MatchLabels)); err != nil {
		return errors.Wrap(err, errListResourceMatchingLabels)
	}

	if len(l.Items) == 0 {
		return errors.Errorf(errFmtResourcesNotFound, rs.Kind, rs.ResourceSelector.MatchLabels)
	}

	for i := range l.Items {
		o := l.Items[i]
		if controllersMustMatch(rs.ResourceSelector) && !meta.HaveSameController(&o, u) {
			continue
		}
		rs.ResourceRef = &v1alpha1.ResourceRef{
			Name: o.GetName(),
		}
		break
	}

	if rs.ResourceRef == nil {
		return errors.Errorf(errFmtResourcesNotFoundWithControllerRef, rs.Kind, rs.ResourceSelector.MatchLabels)
	}

	return nil
}

// ControllersMustMatch returns true if the supplied Selector requires that a
// reference be to a resource whose controller reference matches the
// referencing resource.
func controllersMustMatch(s *v1alpha1.ResourceSelector) bool {
	if s == nil {
		return false
	}

	return s.MatchControllerRef != nil && *s.MatchControllerRef
}

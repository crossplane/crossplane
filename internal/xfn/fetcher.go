/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/internal/proto"
)

// A ExtraResourcesFetcher gets extra resources matching a selector.
type ExtraResourcesFetcher interface {
	Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)
}

// An ExtraResourcesFetcherFn gets extra resources matching the selector.
type ExtraResourcesFetcherFn func(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)

// Fetch gets extra resources matching the selector.
func (fn ExtraResourcesFetcherFn) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	return fn(ctx, rs)
}

// ExistingExtraResourcesFetcher fetches extra resources requested by
// functions using the provided client.Reader.
type ExistingExtraResourcesFetcher struct {
	client client.Reader
}

// NewExistingExtraResourcesFetcher returns a new ExistingExtraResourcesFetcher.
func NewExistingExtraResourcesFetcher(c client.Reader) *ExistingExtraResourcesFetcher {
	return &ExistingExtraResourcesFetcher{client: c}
}

// Fetch fetches resources requested by functions using the provided client.Reader.
func (e *ExistingExtraResourcesFetcher) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	if rs == nil {
		return nil, errors.New(errNilResourceSelector)
	}
	switch match := rs.GetMatch().(type) {
	case *fnv1.ResourceSelector_MatchName:
		// Fetch a single resource.
		r := &kunstructured.Unstructured{}
		r.SetAPIVersion(rs.GetApiVersion())
		r.SetKind(rs.GetKind())
		nn := types.NamespacedName{Name: rs.GetMatchName()}
		err := e.client.Get(ctx, nn, r)
		if kerrors.IsNotFound(err) {
			// The resource doesn't exist. We'll return nil, which the Functions
			// know means that the resource was not found.
			return nil, nil
		}
		if err != nil {
			return nil, errors.Wrap(err, errGetExtraResourceByName)
		}
		o, err := proto.AsStruct(r)
		if err != nil {
			return nil, errors.Wrap(err, errExtraResourceAsStruct)
		}
		return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: o}}}, nil
	case *fnv1.ResourceSelector_MatchLabels:
		// Fetch a list of resources.
		list := &kunstructured.UnstructuredList{}
		list.SetAPIVersion(rs.GetApiVersion())
		list.SetKind(rs.GetKind())

		if err := e.client.List(ctx, list, client.MatchingLabels(match.MatchLabels.GetLabels())); err != nil {
			return nil, errors.Wrap(err, errListExtraResources)
		}

		resources := make([]*fnv1.Resource, len(list.Items))
		for i, r := range list.Items {
			o, err := proto.AsStruct(&r)
			if err != nil {
				return nil, errors.Wrap(err, errExtraResourceAsStruct)
			}
			resources[i] = &fnv1.Resource{Resource: o}
		}

		return &fnv1.Resources{Items: resources}, nil
	}
	return nil, errors.New(errUnknownResourceSelector)
}

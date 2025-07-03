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

package resource

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
)

// TreeClient is the interface to get a Resource with all its children.
type TreeClient interface {
	GetResourceTree(ctx context.Context, root *Resource) (*Resource, error)
}

// GetResource returns the requested Resource, setting any error as Resource.Error.
func GetResource(ctx context.Context, client client.Client, ref *v1.ObjectReference) *Resource {
	result := unstructured.Unstructured{}
	result.SetGroupVersionKind(ref.GroupVersionKind())

	err := client.Get(ctx, xpmeta.NamespacedNameOf(ref), &result)
	if err != nil {
		// If the resource is not found, we still want to return a Resource
		// object with the name and namespace set, so that the caller can
		// still use it.
		result.SetName(ref.Name)
		result.SetNamespace(ref.Namespace)
	}

	return &Resource{Unstructured: result, Error: err}
}

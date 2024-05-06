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

package definition

import (
	"fmt"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

const (
	// compositeResourcesRefsIndex is an index of resourceRefs that are owned
	// by a composite.
	compositeResourcesRefsIndex = "compositeResourcesRefs"
)

var _ client.IndexerFunc = IndexCompositeResourcesRefs

// IndexCompositeResourcesRefs assumes the passed object is a composite. It
// returns keys for every composed resource referenced in the composite.
func IndexCompositeResourcesRefs(o client.Object) []string {
	u, ok := o.(*kunstructured.Unstructured)
	if !ok {
		return nil // should never happen
	}
	xr := composite.Unstructured{Unstructured: *u}
	refs := xr.GetResourceReferences()
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, refKey(ref.Namespace, ref.Name, ref.Kind, ref.APIVersion))
	}
	return keys
}

func refKey(ns, name, kind, apiVersion string) string {
	return fmt.Sprintf("%s.%s.%s.%s", name, ns, kind, apiVersion)
}

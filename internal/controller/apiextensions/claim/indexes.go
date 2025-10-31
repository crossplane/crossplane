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

package claim

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

const xrdByCompositeGVKIndex = "xrdByCompositeGVK"

// XRDByCompositeGVKIndex is the name of an index that indexes XRDs by their
// composite GVK. This allows us to quickly look up the XRD for a given composite resource.
func XRDByCompositeGVKIndex() string {
	return xrdByCompositeGVKIndex
}

// IndexXRDByCompositeGVK returns an IndexerFunc that indexes XRDs by their
// composite GroupVersionKind. This allows efficient lookup of an XRD given
// the GVK of a composite resource.
func IndexXRDByCompositeGVK() client.IndexerFunc {
	return func(o client.Object) []string {
		xrd, ok := o.(*v1.CompositeResourceDefinition)
		if !ok {
			return nil // should never happen
		}

		gvk := xrd.GetCompositeGroupVersionKind()
		return []string{compositeGVKKey(gvk.Group, gvk.Kind)}
	}
}

func compositeGVKKey(group, kind string) string {
	return fmt.Sprintf("%s.%s", group, kind)
}

// compositeGVKKeyFor returns the index key for the given composite GVK.
func compositeGVKKeyFor(gvk schema.GroupVersionKind) string {
	return compositeGVKKey(gvk.Group, gvk.Kind)
}

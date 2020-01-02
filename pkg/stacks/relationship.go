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

package stacks

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// Labels used to track ownership across namespaces and scopes.
const (
	LabelParentGroup     = "core.crossplane.io/parent-group"
	LabelParentVersion   = "core.crossplane.io/parent-version"
	LabelParentKind      = "core.crossplane.io/parent-kind"
	LabelParentNamespace = "core.crossplane.io/parent-namespace"
	LabelParentName      = "core.crossplane.io/parent-name"
	LabelParentUID       = "core.crossplane.io/parent-uid"
)

// KindlyIdentifier implementations provide the means to access the Name,
// Namespace, GVK, and UID of a resource
type KindlyIdentifier interface {
	GetName() string
	GetNamespace() string
	GetUID() types.UID

	GroupVersionKind() schema.GroupVersionKind
}

// ParentLabels returns a map of labels referring to the given resource
func ParentLabels(i KindlyIdentifier) map[string]string {
	gvk := i.GroupVersionKind()

	labels := map[string]string{
		LabelParentGroup:     gvk.Group,
		LabelParentVersion:   gvk.Version,
		LabelParentKind:      gvk.Kind,
		LabelParentNamespace: i.GetNamespace(),
		LabelParentName:      i.GetName(),
		LabelParentUID:       string(i.GetUID()),
	}
	return labels
}

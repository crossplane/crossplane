/*
Copyright 2019 The Crossplane Authors.

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

package packages

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane/pkg/packages/truncate"
)

// Labels used to track ownership across namespaces and scopes.
const (
	LabelParentGroup     = "core.crossplane.io/parent-group"
	LabelParentVersion   = "core.crossplane.io/parent-version"
	LabelParentKind      = "core.crossplane.io/parent-kind"
	LabelParentNamespace = "core.crossplane.io/parent-namespace"
	LabelParentName      = "core.crossplane.io/parent-name"

	LabelMultiParentPrefix = "parent.packages.crossplane.io/"

	LabelMultiParentNSFormat = LabelMultiParentPrefix + "%s"

	// LabelMultiParentFormat defines the format for combining a
	// LabelMultiParentNSFormat with a named resource
	// Example:
	// fmt.Sprintf(LabelMultiParentFormat,
	//   fmt.Sprintf(LabelMultiParentNSFormat,
	//   nsName,
	// ), resourceName)
	LabelMultiParentFormat = "%s-%s"

	// preserveNSLength is the number of characters using the label name that
	// will be dedicated to identifying the namespace. This length will include
	// truncation characters if the namespace exceeds this length. example:
	// parent.packages.crossplane.io/{up to 32 chars of NS}-{Name}
	//
	// NOTE: Changes to this length will prevent resources from be discovered
	// and could lead to the deletion or recreation of resources.
	preserveNSLength = 32
)

// KindlyIdentifier implementations provide the means to access the Name,
// Namespace, GVK, and UID of a resource
type KindlyIdentifier interface {
	GetName() string
	GetNamespace() string
	GetUID() types.UID

	GroupVersionKind() schema.GroupVersionKind
}

// MultiParentLabelPrefix returns the NS specific prefix of a multi-parent label
// for resources co-owned by a set of Packages.
//
// This prefix is suitable for identifying resources labeled within a given
// namespace. The prefix may include a predictable truncation suffix if the
// namespace exceeds 32 characters. This truncation length permits another 32
// characters for a (potentially truncated) resource name to be appended to the
// label.
//
// Example: MultiParentLabelPrefix(resource.SetNamespace("foo")) ->
//   "parent.packages.crossplane.io/foo"
//
// A namespace name over 32 characters will be truncated in the returned label
// prefix.
func MultiParentLabelPrefix(packageParent metav1.Object) string {
	ns := packageParent.GetNamespace()

	// guaranteed not to error based on the lengths of values we are supplying
	truncated, _ := truncate.Truncate(ns, preserveNSLength, truncate.DefaultSuffixLength)
	return fmt.Sprintf(LabelMultiParentNSFormat, truncated)
}

// MultiParentLabel returns a label name identifying the namespaced name of the
// package resource that co-owns another resource
//
// The label returned is based on the MultiParentLabelPrefix, which may include
// a truncation suffix, and is then potentially truncated again to fit in the
// complete label length restrictions.
//
// Example: MultiParentLabel(resource.SetNamespace("foo").SetName("bar").) ->
//   "parent.packages.crossplane.io/foo-bar"
//
// A namespace name over 32 characters will be truncated in the returned label
// prefix, if the namespace and name, combined exceed 63 characters an
// additional truncation will be included.
func MultiParentLabel(packageParent metav1.Object) string {
	prefix := MultiParentLabelPrefix(packageParent)

	// guaranteed at least 2 parts based on LabelMultiParentNSFormat
	prefixParts := strings.SplitN(prefix, "/", 2)

	n := packageParent.GetName()
	full := fmt.Sprintf(LabelMultiParentFormat, prefixParts[1], n)

	truncated := fmt.Sprintf(LabelMultiParentNSFormat, truncate.LabelName(full))

	return truncated
}

// ParentLabels returns a map of labels referring to the given resource
func ParentLabels(i KindlyIdentifier) map[string]string {
	gvk := i.GroupVersionKind()

	// namespaces and names may be 253 characters, while label values may not
	// exceed 63
	labels := map[string]string{
		LabelParentGroup:     gvk.Group,
		LabelParentVersion:   gvk.Version,
		LabelParentKind:      gvk.Kind,
		LabelParentNamespace: truncate.LabelValue(i.GetNamespace()),
		LabelParentName:      truncate.LabelValue(i.GetName()),
	}
	return labels
}

// HasPrefixedLabel checks if any label on an Object starts with any of the
// provided prefixes
func HasPrefixedLabel(obj metav1.Object, prefixes ...string) bool {
	for k := range obj.GetLabels() {
		for _, prefix := range prefixes {
			if strings.HasPrefix(k, prefix) {
				return true
			}
		}
	}
	return false
}

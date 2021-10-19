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

package v1

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EnabledAPIs represents list of enabled GVKs for a package.
type EnabledAPIs []string

// String returns items in this EnabledAPIs separated via commas.
// This is the format expected to be used with the `--enabled-gvks`
// command-line options of providers.
func (apis EnabledAPIs) String() string {
	return strings.Join(apis, ",")
}

// Digest returns the SHA-256 digest for this EnabledAPIs.
// It's stable in terms of duplicates and reordering.
func (apis EnabledAPIs) Digest() string {
	if len(apis) == 0 {
		return ""
	}
	// remove duplicate expressions and sort
	result := make(EnabledAPIs, 0, len(apis))
	m := map[string]struct{}{}
	for _, r := range apis {
		if _, ok := m[r]; !ok {
			result = append(result, r)
			m[r] = struct{}{}
		}
	}
	sort.Strings(result)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(result.String())))
}

// Filter drops elements from objs whose GVK does not match any
// regular expression specified in this EnabledAPIs.
func (apis EnabledAPIs) Filter(objs []runtime.Object) ([]runtime.Object, error) {
	if len(apis) == 0 {
		return objs, nil
	}

	filtered := make([]runtime.Object, 0, len(objs))
	for _, o := range objs {
		gvk := o.GetObjectKind().GroupVersionKind()
		gvkArr := []schema.GroupVersionKind{gvk}
		if crd, ok := o.(*v1.CustomResourceDefinition); ok {
			for _, v := range crd.Spec.Versions {
				gvkArr = append(gvkArr, schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: v.Name,
					Kind:    crd.Spec.Names.Kind,
				})
			}
		}

		for _, gvk := range gvkArr {
			gvkStr := fmt.Sprintf(fmtGVK, gvk.Group, gvk.Version, gvk.Kind)
			found := false
			for _, r := range apis {
				ok, err := regexp.MatchString(r, gvkStr)
				if err != nil {
					return nil, errors.Wrap(err, errMatchGVK)
				}
				if ok {
					found = true
					break
				}
			}
			if found {
				filtered = append(filtered, o)
				break
			}
		}
	}
	return filtered, nil
}

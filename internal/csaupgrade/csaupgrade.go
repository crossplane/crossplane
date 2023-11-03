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

// Package csaupgrade contains helper functions
// for migrating client-side to server-side apply
package csaupgrade

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type filterFn func(e v1.ManagedFieldsEntry) bool

var (
	// field manager names used by previous Crossplane versions
	csaManagerNames = []string{"Go-http-client", "crossplane"}

	// SkipSubresources skips migration of field managers set on subresource
	SkipSubresources = func(e v1.ManagedFieldsEntry) bool {
		return e.Subresource != ""
	}

	// OnlySubresources migrates field manager on subresources only
	OnlySubresources = func(e v1.ManagedFieldsEntry) bool {
		return e.Subresource == ""
	}

	// All migrates field manager both on main and subresources
	All = func(e v1.ManagedFieldsEntry) bool {
		return false
	}
)

// MaybeFixFieldOwnership migrates field managers from client-side
// to server-side approach, and returns true if the migration was performed.
// For the background, check https://github.com/kubernetes/kubernetes/issues/99003
// Managed fields owner key is a pair (manager name, used operation).
// Previous versions of Crossplane create a composite using patching applicator.
// Even if the server-side apply is not used, api server derives manager name
// from the submitted user agent (see net/http/request.go).
// After Crossplane update, we need to replace the ownership so that
// field removals can be propagated properly.
// In order to fix that, we need to manually change operation to "Apply",
// and the manager name, before the first composite patch is sent to k8s api server.
// Returns true if the ownership was fixed.
// TODO: this code can be removed once Crossplane v1.13 is not longer supported
func MaybeFixFieldOwnership(obj *unstructured.Unstructured, ssaManagerName string, filter filterFn) bool {
	mfs := obj.GetManagedFields()
	umfs := make([]v1.ManagedFieldsEntry, len(mfs))
	copy(umfs, mfs)
	fixed := false
	for j := range csaManagerNames {
		for i := range umfs {
			if umfs[i].Manager == ssaManagerName {
				return fixed
			}
			if filter(umfs[i]) {
				continue
			}

			if umfs[i].Manager == csaManagerNames[j] && umfs[i].Operation == v1.ManagedFieldsOperationUpdate {
				umfs[i].Operation = v1.ManagedFieldsOperationApply
				umfs[i].Manager = ssaManagerName
				fixed = true
			}
		}
		if fixed {
			obj.SetManagedFields(umfs)
		}
	}

	return fixed

}

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

// Package ssa contains utilities for working with server-side apply.
package ssa

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// A FieldManagerMatcher returns true if the supplied field manager name matches
// the criteria for the SSA field manager that should own an object's fields.
type FieldManagerMatcher func(manager string) bool

// ExactMatch returns a FieldManagerMatcher that matches field managers with the
// exact supplied name.
func ExactMatch(name string) FieldManagerMatcher {
	return func(manager string) bool {
		return manager == name
	}
}

// PrefixMatch returns a FieldManagerMatcher that matches field managers whose
// name starts with the supplied prefix.
func PrefixMatch(prefix string) FieldManagerMatcher {
	return func(manager string) bool {
		return strings.HasPrefix(manager, prefix)
	}
}

// A ManagedFieldsUpgrader upgrades an objects managed fields from client-side
// apply to server-side apply. This is necessary when an object was previously
// managed using client-side apply, but should now be managed using server-side
// apply. See https://github.com/kubernetes/kubernetes/issues/99003 for details.
type ManagedFieldsUpgrader interface {
	Upgrade(ctx context.Context, obj client.Object) error
}

// A NopManagedFieldsUpgrader does nothing.
type NopManagedFieldsUpgrader struct{}

// Upgrade does nothing.
func (u *NopManagedFieldsUpgrader) Upgrade(_ context.Context, _ client.Object) error {
	return nil
}

// A PatchingManagedFieldsUpgrader uses a JSON patch to upgrade an object's
// managed fields from client-side to server-side apply. The upgrade is a no-op
// if the object does not need upgrading.
type PatchingManagedFieldsUpgrader struct {
	client  client.Writer
	matcher FieldManagerMatcher
}

// NewPatchingManagedFieldsUpgrader returns a ManagedFieldsUpgrader that uses a
// JSON patch to upgrade and object's managed fields from client-side to
// server-side apply.
func NewPatchingManagedFieldsUpgrader(w client.Writer, m FieldManagerMatcher) *PatchingManagedFieldsUpgrader {
	return &PatchingManagedFieldsUpgrader{client: w, matcher: m}
}

// Upgrade the supplied object's field managers from client-side to server-side
// apply.
//
// This is a multi-step process.
//
// Step 1: All fields are owned by various managers, potentially including
// client-side apply managers or no manager at all. This represents all fields
// set on the object before the controller started using server-side apply.
//
// Step 2: Upgrade is called for the first time. We clear all field managers.
//
// Step 3: The controller server-side applies its fully specified intent
// as the supplied field manager. This becomes the manager of all the fields
// that are part of the controller's fully specified intent. All existing fields
// the controller didn't specify become owned by a special manager -
// 'before-first-apply', operation 'Update'.
//
// Step 4: Upgrade is called for the second time. It deletes the
// 'before-first-apply' field manager entry. Only the controller's field manager
// remains.
func (u *PatchingManagedFieldsUpgrader) Upgrade(ctx context.Context, obj client.Object) error {
	// The object doesn't exist, nothing to upgrade.
	if !meta.WasCreated(obj) {
		return nil
	}

	foundSSA := false
	foundBFA := false
	idxBFA := -1

	for i, e := range obj.GetManagedFields() {
		if u.matcher(e.Manager) {
			foundSSA = true
		}

		if e.Manager == "before-first-apply" {
			foundBFA = true
			idxBFA = i
		}
	}

	switch {
	// If our SSA field manager exists and the before-first-apply field manager
	// doesn't, we've already done the upgrade. Don't do it again.
	case foundSSA && !foundBFA:
		return nil

	// We found our SSA field manager but also before-first-apply. It should now
	// be safe to delete before-first-apply.
	case foundSSA && foundBFA:
		p := fmt.Appendf([]byte{}, `[
			{"op":"remove","path":"/metadata/managedFields/%d"},
			{"op":"replace","path":"/metadata/resourceVersion","value":"%s"}
		]`, idxBFA, obj.GetResourceVersion())

		return errors.Wrap(resource.IgnoreNotFound(u.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, p))), "cannot remove before-first-apply from field managers")

	// We didn't find our SSA field manager or the before-first-apply field
	// manager. This means we haven't started the upgrade. The first thing we
	// want to do is clear all managed fields. After we do this we'll let our
	// SSA field manager apply the fields it cares about. The result will be
	// that our SSA field manager shares ownership with a new manager named
	// 'before-first-apply'.
	default:
		p := fmt.Appendf([]byte{}, `[
			{"op":"replace","path": "/metadata/managedFields","value": [{}]},
			{"op":"replace","path":"/metadata/resourceVersion","value":"%s"}
		]`, obj.GetResourceVersion())

		return errors.Wrap(resource.IgnoreNotFound(u.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, p))), "cannot clear field managers")
	}
}

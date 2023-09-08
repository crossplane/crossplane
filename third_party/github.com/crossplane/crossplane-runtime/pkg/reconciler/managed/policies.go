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

package managed

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ManagementPoliciesResolver is used to perform management policy checks
// based on the management policy and if the management policy feature is enabled.
type ManagementPoliciesResolver struct {
	enabled            bool
	supportedPolicies  []sets.Set[xpv1.ManagementAction]
	managementPolicies sets.Set[xpv1.ManagementAction]
	deletionPolicy     xpv1.DeletionPolicy
}

// A ManagementPoliciesResolverOption configures a ManagementPoliciesResolver.
type ManagementPoliciesResolverOption func(*ManagementPoliciesResolver)

// WithSupportedManagementPolicies sets the supported management policies.
func WithSupportedManagementPolicies(supportedManagementPolicies []sets.Set[xpv1.ManagementAction]) ManagementPoliciesResolverOption {
	return func(r *ManagementPoliciesResolver) {
		r.supportedPolicies = supportedManagementPolicies
	}
}

func defaultSupportedManagementPolicies() []sets.Set[xpv1.ManagementAction] {
	return []sets.Set[xpv1.ManagementAction]{
		// Default (all), the standard behaviour of crossplane in which all
		// reconciler actions are done.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll),
		// All actions explicitly set, the same as default.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionLateInitialize, xpv1.ManagementActionDelete),
		// ObserveOnly, just observe action is done, the external resource is
		// considered as read-only.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
		// Pause, no action is being done. Alternative to setting the pause
		// annotation.
		sets.New[xpv1.ManagementAction](),
		// No LateInitialize filling in the spec.forProvider, allowing some
		// external resource fields to be managed externally.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionDelete),
		// No Delete, the external resource is not deleted when the managed
		// resource is deleted.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionLateInitialize),
		// No Delete and no LateInitialize, the external resource is not deleted
		// when the managed resource is deleted and the spec.forProvider is not
		// late initialized.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate),
		// No Update, the external resource is not updated when the managed
		// resource is updated. Useful for immutable external resources.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete, xpv1.ManagementActionLateInitialize),
		// No Update and no Delete, the external resource is not updated
		// when the managed resource is updated and the external resource
		// is not deleted when the managed resource is deleted.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionLateInitialize),
		// No Update and no LateInitialize, the external resource is not updated
		// when the managed resource is updated and the spec.forProvider is not
		// late initialized.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete),
		// No Update, no Delete and no LateInitialize, the external resource is
		// not updated when the managed resource is updated, the external resource
		// is not deleted when the managed resource is deleted and the
		// spec.forProvider is not late initialized.
		sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate),
	}
}

// NewManagementPoliciesResolver returns an ManagementPolicyChecker based
// on the management policies and if the management policies feature
// is enabled.
func NewManagementPoliciesResolver(managementPolicyEnabled bool, managementPolicy xpv1.ManagementPolicies, deletionPolicy xpv1.DeletionPolicy, o ...ManagementPoliciesResolverOption) ManagementPoliciesChecker {
	r := &ManagementPoliciesResolver{
		enabled:            managementPolicyEnabled,
		supportedPolicies:  defaultSupportedManagementPolicies(),
		managementPolicies: sets.New[xpv1.ManagementAction](managementPolicy...),
		deletionPolicy:     deletionPolicy,
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Validate checks if the management policy is valid.
// If the management policy feature is disabled, but uses a non-default value,
// it returns an error.
// If the management policy feature is enabled, but uses a non-supported value,
// it returns an error.
func (m *ManagementPoliciesResolver) Validate() error {
	// check if its disabled, but uses a non-default value.
	if !m.enabled {
		if !m.managementPolicies.Equal(sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll)) && m.managementPolicies.Len() != 0 {
			return fmt.Errorf(errFmtManagementPolicyNonDefault, m.managementPolicies.UnsortedList())
		}
		// if its just disabled we don't care about supported policies
		return nil
	}

	// check if the policy is a non-supported combination
	for _, p := range m.supportedPolicies {
		if p.Equal(m.managementPolicies) {
			return nil
		}
	}
	return fmt.Errorf(errFmtManagementPolicyNotSupported, m.managementPolicies.UnsortedList())
}

// IsPaused returns true if the management policy is empty and the
// management policies feature is enabled
func (m *ManagementPoliciesResolver) IsPaused() bool {
	if !m.enabled {
		return false
	}
	return m.managementPolicies.Len() == 0
}

// ShouldCreate returns true if the Create action is allowed.
// If the management policy feature is disabled, it returns true.
func (m *ManagementPoliciesResolver) ShouldCreate() bool {
	if !m.enabled {
		return true
	}
	return m.managementPolicies.HasAny(xpv1.ManagementActionCreate, xpv1.ManagementActionAll)
}

// ShouldUpdate returns true if the Update action is allowed.
// If the management policy feature is disabled, it returns true.
func (m *ManagementPoliciesResolver) ShouldUpdate() bool {
	if !m.enabled {
		return true
	}
	return m.managementPolicies.HasAny(xpv1.ManagementActionUpdate, xpv1.ManagementActionAll)
}

// ShouldLateInitialize returns true if the LateInitialize action is allowed.
// If the management policy feature is disabled, it returns true.
func (m *ManagementPoliciesResolver) ShouldLateInitialize() bool {
	if !m.enabled {
		return true
	}
	return m.managementPolicies.HasAny(xpv1.ManagementActionLateInitialize, xpv1.ManagementActionAll)
}

// ShouldOnlyObserve returns true if the Observe action is allowed and all
// other actions are not allowed. If the management policy feature is disabled,
// it returns false.
func (m *ManagementPoliciesResolver) ShouldOnlyObserve() bool {
	if !m.enabled {
		return false
	}
	return m.managementPolicies.Equal(sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve))
}

// ShouldDelete returns true based on the combination of the deletionPolicy and
// the managementPolicies. If the management policy feature is disabled, it
// returns true if the deletionPolicy is set to "Delete". Otherwise, it checks
// which field is set to a non-default value and makes a decision based on that.
// We need to be careful until we completely remove the deletionPolicy in favor
// of managementPolicies which conflict with the deletionPolicy regarding
// deleting of the external resource. This function implements the proposal in
// the Ignore Changes design doc under the "Deprecation of `deletionPolicy`".
func (m *ManagementPoliciesResolver) ShouldDelete() bool {
	if !m.enabled {
		return m.deletionPolicy != xpv1.DeletionOrphan
	}

	// delete external resource if both the deletionPolicy and the
	// managementPolicies are set to delete
	if m.deletionPolicy == xpv1.DeletionDelete && m.managementPolicies.HasAny(xpv1.ManagementActionDelete, xpv1.ManagementActionAll) {
		return true
	}
	// if the managementPolicies is not default, and it contains the deletion
	// action, we should delete the external resource
	if !m.managementPolicies.Equal(sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll)) && m.managementPolicies.Has(xpv1.ManagementActionDelete) {
		return true
	}

	// For all other cases, we should orphan the external resource.
	// Obvious cases:
	// DeletionOrphan && ManagementPolicies without Delete Action
	// Conflicting cases:
	// DeletionOrphan && Management Policy ["*"] (obeys non-default configuration)
	// DeletionDelete && ManagementPolicies that does not include the Delete
	// Action (obeys non-default configuration)
	return false
}

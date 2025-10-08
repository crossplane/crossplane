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

// Package roles provides shared utilities for RBAC role management.
package roles

import (
	"strings"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
)

const (
	// NameSuffixEdit is the suffix for edit aggregate roles.
	NameSuffixEdit = ":aggregate-to-edit"

	// NameSuffixView is the suffix for view aggregate roles.
	NameSuffixView = ":aggregate-to-view"

	// KeyAggregateToCrossplane is the label key for aggregating to crossplane role.
	KeyAggregateToCrossplane = "rbac.crossplane.io/aggregate-to-crossplane"

	// KeyAggregateToAdmin is the label key for aggregating to admin role.
	KeyAggregateToAdmin = "rbac.crossplane.io/aggregate-to-admin"

	// KeyAggregateToEdit is the label key for aggregating to edit role.
	KeyAggregateToEdit = "rbac.crossplane.io/aggregate-to-edit"

	// KeyAggregateToView is the label key for aggregating to view role.
	KeyAggregateToView = "rbac.crossplane.io/aggregate-to-view"

	// ValTrue is the value for true labels.
	ValTrue = "true"

	// SuffixStatus is the suffix for status subresources.
	SuffixStatus = "/status"
)

//nolint:gochecknoglobals // We treat these as constants.
var (
	// VerbsEdit are the verbs for edit permissions.
	VerbsEdit = []string{rbacv1.VerbAll}

	// VerbsView are the verbs for view permissions.
	VerbsView = []string{"get", "list", "watch"}
)

// A Resource is a Kubernetes API resource.
type Resource struct {
	// Group is the unversioned API group of this resource.
	Group string

	// Plural is the plural name of this resource.
	Plural string
}

// DefinedResources returns the resources defined by the supplied references.
func DefinedResources(refs []xpv1.TypedReference) []Resource {
	out := make([]Resource, 0, len(refs))
	for _, ref := range refs {
		// This would only return an error if the APIVersion contained more than
		// one "/". This should be impossible, but if it somehow happens we'll
		// just skip this resource since it can't be a CRD.
		gv, _ := schema.ParseGroupVersion(ref.APIVersion)

		// We're only concerned with CRDs or MRDs.
		switch {
		case gv.Group == apiextensions.GroupName && ref.Kind == "CustomResourceDefinition":
		// Do the work!
		case gv.Group == v1alpha1.Group && ref.Kind == v1alpha1.ManagedResourceDefinitionKind:
		// Do the work!
		default:
			// Filter out the non CRD or MRD.
			continue
		}

		p, g, valid := strings.Cut(ref.Name, ".")
		if !valid {
			// This shouldn't be possible - CRDs must be named <plural>.<group>.
			continue
		}

		out = append(out, Resource{Group: g, Plural: p})
	}

	return out
}

// ClusterRolesDiffer returns true if the supplied objects are different
// ClusterRoles. We consider ClusterRoles to be different if their labels and
// rules do not match.
func ClusterRolesDiffer(current, desired runtime.Object) bool {
	// Calling this with anything but ClusterRoles is a programming error. If it
	// happens, we probably do want to panic.
	c := current.(*rbacv1.ClusterRole) //nolint:forcetypeassert // See above.
	d := desired.(*rbacv1.ClusterRole) //nolint:forcetypeassert // See above.

	return !cmp.Equal(c.GetLabels(), d.GetLabels()) || !cmp.Equal(c.Rules, d.Rules)
}

// WithVerbs returns a copy of the supplied rules with the supplied verbs.
func WithVerbs(r []rbacv1.PolicyRule, verbs []string) []rbacv1.PolicyRule {
	verbal := make([]rbacv1.PolicyRule, len(r))
	for i := range r {
		verbal[i] = r[i]
		verbal[i].Verbs = verbs
	}

	return verbal
}

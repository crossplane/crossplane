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

package roles

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errGetClusterRole           = "cannot get ClusterRole"
	errExpandClusterRoleRules   = "cannot expand ClusterRole rules"
	errExpandPermissionRequests = "cannot expand PermissionRequests"
)

const (
	wildcard = "*"

	pathPrefixURL      = "url"
	pathPrefixResource = "resource"
)

// A path represents a rule within the rule tree.
type path []string

// A node within the rule tree. Each node indicates whether the path should be
// allowed. In practice only leaves (i.e. Verbs) should have this set.
type node struct {
	allowed  bool
	children map[string]*node
}

// newNode initialises and returns a Node.
func newNode() *node {
	return &node{children: map[string]*node{}}
}

// Allow the supplied path.
func (n *node) Allow(p path) {
	if len(p) == 0 {
		n.allowed = true
		return
	}

	k := p[0]
	if _, ok := n.children[k]; !ok {
		n.children[k] = newNode()
	}
	n.children[k].Allow(p[1:])
}

// Allowed returns true if the supplied path is allowed.
func (n *node) Allowed(p path) bool {
	if len(p) == 0 {
		return false
	}

	// TODO(negz): A NonResourceURL can't be a wildcard, but can have a final
	// segment that is a wildcard; e.g. /apis/*. We don't currently support
	// this.
	for _, k := range []string{p[0], wildcard} {
		if c, ok := n.children[k]; ok {
			if c.allowed {
				return true
			}
			if c.Allowed(p[1:]) {
				return true
			}
		}
	}
	return false
}

// A Rule represents a single, granular RBAC rule.
type Rule struct {
	// The API group of this resource. The empty string denotes the core
	// Kubernetes API group. '*' represents any API group.
	APIGroup string

	// The resource in question. '*' represents any resource.
	Resource string

	// The name of the resource. Unlike the rbacv1 API, we use '*' to represent
	// any resource name.
	ResourceName string

	// A non-resource URL. Mutually exclusive with the above resource fields.
	NonResourceURL string

	// The verb this rule allows.
	Verb string
}

func (r Rule) String() string {
	if r.NonResourceURL != "" {
		return fmt.Sprintf("{nonResourceURL: %q, verb: %q}", r.NonResourceURL, r.Verb)
	}
	return fmt.Sprintf("{apiGroup: %q, resource: %q, resourceName: %q, verb: %q}", r.APIGroup, r.Resource, r.ResourceName, r.Verb)
}

// path of the rule, for use in a tree.
func (r Rule) path() path {
	if r.NonResourceURL != "" {
		return path{pathPrefixURL, r.NonResourceURL, r.Verb}
	}
	return path{pathPrefixResource, r.APIGroup, r.Resource, r.ResourceName, r.Verb}
}

// Expand RBAC policy rules into our granular rules.
func Expand(ctx context.Context, rs ...rbacv1.PolicyRule) ([]Rule, error) { //nolint:gocognit // Granular rules are inherently complex.
	out := make([]Rule, 0, len(rs))
	for _, r := range rs {
		for _, u := range r.NonResourceURLs {
			for _, v := range r.Verbs {
				// exit if ctx is done
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				out = append(out, Rule{NonResourceURL: u, Verb: v})
			}
		}

		// ResourceNames are somewhat unique in rbacv1.PolicyRule in that no
		// names means all names. APIGroups and Resources use the wildcard to
		// represent that. We use the wildcard here too to simplify our logic.
		names := r.ResourceNames
		if len(names) < 1 {
			names = []string{wildcard}
		}

		for _, g := range r.APIGroups {
			for _, rsc := range r.Resources {
				for _, n := range names {
					for _, v := range r.Verbs {
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						default:
						}
						out = append(out, Rule{APIGroup: g, Resource: rsc, ResourceName: n, Verb: v})
					}
				}
			}
		}
	}
	return out, nil
}

// A ClusterRoleBackedValidator is a PermissionRequestsValidator that validates
// permission requests by comparing them to an RBAC ClusterRole. The validator
// will reject any permission that is not permitted by the ClusterRole.
type ClusterRoleBackedValidator struct {
	client client.Client
	name   string
}

// NewClusterRoleBackedValidator creates a ClusterRoleBackedValidator backed by
// the named RBAC ClusterRole.
func NewClusterRoleBackedValidator(c client.Client, roleName string) *ClusterRoleBackedValidator {
	return &ClusterRoleBackedValidator{client: c, name: roleName}
}

// ValidatePermissionRequests against the ClusterRole, returning the list of rejected rules.
func (v *ClusterRoleBackedValidator) ValidatePermissionRequests(ctx context.Context, requests ...rbacv1.PolicyRule) ([]Rule, error) {
	cr := &rbacv1.ClusterRole{}
	if err := v.client.Get(ctx, types.NamespacedName{Name: v.name}, cr); err != nil {
		return nil, errors.Wrap(err, errGetClusterRole)
	}

	t := newNode()
	expandedCrRules, err := Expand(ctx, cr.Rules...)
	if err != nil {
		return nil, errors.Wrap(err, errExpandClusterRoleRules)
	}
	for _, rule := range expandedCrRules {
		t.Allow(rule.path())
	}

	rejected := make([]Rule, 0)
	expandedRequests, err := Expand(ctx, requests...)
	if err != nil {
		return nil, errors.Wrap(err, errExpandPermissionRequests)
	}
	for _, rule := range expandedRequests {
		if !t.Allowed(rule.path()) {
			rejected = append(rejected, rule)
		}
	}

	return rejected, nil
}

// VerySecureValidator is a PermissionRequestsValidatorFn that rejects all
// requested permissions.
func VerySecureValidator(ctx context.Context, requests ...rbacv1.PolicyRule) ([]Rule, error) {
	return Expand(ctx, requests...)
}

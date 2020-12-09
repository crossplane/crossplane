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

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Error strings.
const (
	errGetClusterRole = "cannot get ClusterRole"
)

const (
	wildcard = "*"

	pathPrefixURL      = "url"
	pathPrefixResource = "resource"
)

// A Path represents a rule within the rule tree.
type Path []string

// A Node within the rule tree. Each node indicates whether the path should be
// allowed. In practice only leaves (i.e. Verbs) should have this set.
type Node struct {
	allowed  bool
	children map[string]*Node
}

// NewNode initialises and returns a Node.
func NewNode() *Node {
	return &Node{children: map[string]*Node{}}
}

// Allow the supplied path.
func (n *Node) Allow(p Path) {
	if len(p) == 0 {
		n.allowed = true
		return
	}

	k := p[0]
	if _, ok := n.children[k]; !ok {
		n.children[k] = NewNode()
	}
	n.children[k].Allow(p[1:])
}

// Allowed returns true if the supplied path is allowed.
func (n *Node) Allowed(p Path) bool {
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

// Path of the rule, for use in a tree.
func (r Rule) Path() Path {
	if r.NonResourceURL != "" {
		return Path{pathPrefixURL, r.NonResourceURL, r.Verb}
	}
	return Path{pathPrefixResource, r.APIGroup, r.Resource, r.ResourceName, r.Verb}
}

// Expand RBAC policy rules into our granular rules.
func Expand(rs ...rbacv1.PolicyRule) []Rule {
	out := make([]Rule, 0, len(rs))
	for _, r := range rs {
		if len(r.NonResourceURLs) > 0 {
			for _, u := range r.NonResourceURLs {
				for _, v := range r.Verbs {
					out = append(out, Rule{NonResourceURL: u, Verb: v})
				}
			}
			continue
		}

		names := r.ResourceNames
		if len(names) < 1 {
			names = []string{"*"}
		}

		for _, g := range r.APIGroups {
			for _, rsc := range r.Resources {
				for _, n := range names {
					for _, v := range r.Verbs {
						out = append(out, Rule{APIGroup: g, Resource: rsc, ResourceName: n, Verb: v})
					}
				}
			}
		}
	}
	return out
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

// ValidatePermissionRequests against the ClusterRole.
func (v *ClusterRoleBackedValidator) ValidatePermissionRequests(ctx context.Context, requests ...rbacv1.PolicyRule) ([]Rule, error) {
	cr := &rbacv1.ClusterRole{}
	if err := v.client.Get(ctx, types.NamespacedName{Name: v.name}, cr); err != nil {
		return nil, errors.Wrap(err, errGetClusterRole)
	}

	t := NewNode()
	for _, rule := range Expand(cr.Rules...) {
		t.Allow(rule.Path())
	}

	rejected := make([]Rule, 0)
	for _, rule := range Expand(requests...) {
		if !t.Allowed(rule.Path()) {
			rejected = append(rejected, rule)
		}
	}

	return rejected, nil
}

// VerySecureValidator is a PermissionRequestsValidatorFn that rejects all
// requested permissions.
func VerySecureValidator(ctx context.Context, requests ...rbacv1.PolicyRule) ([]Rule, error) {
	return Expand(requests...), nil
}

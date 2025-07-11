/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// A CapabilityChecker checks whether the named functions have all the required
// capabilities. It returns an error if any function is missing any capability.
type CapabilityChecker interface {
	CheckCapabilities(ctx context.Context, caps []string, names ...string) error
}

// A CapabilityCheckerFn is a function that can check capabilities.
type CapabilityCheckerFn func(ctx context.Context, caps []string, names ...string) error

// CheckCapabilities checks the capabilities of the named functions.
func (fn CapabilityCheckerFn) CheckCapabilities(ctx context.Context, caps []string, names ...string) error {
	return fn(ctx, caps, names...)
}

// A RevisionCapabilityChecker is a CapabilityChecker that uses FunctionRevisions
// to check capabilities.
type RevisionCapabilityChecker struct {
	client client.Reader
}

// NewRevisionCapabilityChecker returns a new RevisionCapabilityChecker.
func NewRevisionCapabilityChecker(c client.Reader) *RevisionCapabilityChecker {
	return &RevisionCapabilityChecker{client: c}
}

// CheckCapabilities returns nil if all the named functions have all the
// required capabilities.
func (c *RevisionCapabilityChecker) CheckCapabilities(ctx context.Context, caps []string, names ...string) error {
	l := &pkgv1.FunctionRevisionList{}
	if err := c.client.List(ctx, l); err != nil {
		return errors.Wrap(err, "cannot list FunctionRevisions")
	}

	check := map[string]bool{}
	for _, name := range names {
		check[name] = true
	}

	for _, rev := range l.Items {
		// We only want to check the active revision.
		if rev.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		pkgName := rev.GetLabels()[pkgv1.LabelParentPackage]
		// We only want to check revisions of the named packages.
		if !check[pkgName] {
			continue
		}

		has := map[string]bool{}
		for _, cap := range rev.GetCapabilities() {
			has[cap] = true
		}

		missing := make([]string, 0)
		for _, cap := range caps {
			if !has[cap] {
				missing = append(missing, cap)
			}
		}

		if len(missing) > 0 {
			return errors.Errorf("function %q (active revision %q) is missing required capabilities: %s", pkgName, rev.GetName(), strings.Join(missing, ", "))
		}
	}

	return nil
}

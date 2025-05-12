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

package manager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// RevisionActivator is responsible for activating revisions of a package
// according to the package's revision activation policy.
type RevisionActivator interface {
	ActivateRevisions(ctx context.Context, p v1.Package, revs []v1.PackageRevision) ([]v1.PackageRevision, error)
}

// SingleRevisionActivator sets the desired activation state of package
// revisions based on the package's configuration, making only one revision
// active at any given time (i.e., ignoring activeRevisionLimit).
type SingleRevisionActivator struct {
	client resource.ClientApplicator
}

// NewSingleRevisionActivator returns a new PackageRevisionActivator.
func NewSingleRevisionActivator(c client.Client) *SingleRevisionActivator {
	return &SingleRevisionActivator{
		client: resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)},
	}
}

// ActivateRevisions activates revisions based on the package's configuration.
func (a *SingleRevisionActivator) ActivateRevisions(ctx context.Context, p v1.Package, revs []v1.PackageRevision) ([]v1.PackageRevision, error) {
	if p.GetActivationPolicy() != nil && *p.GetActivationPolicy() == v1.ManualActivation {
		// Activation policy is manual - it's up to the user to
		// activate/deactivate revisions.
		return revs, nil
	}

	// Find the current revision and mark all other revisions inactive. We do
	// this before marking the current revision active to ensure only one
	// revision is active at a time.
	var current v1.PackageRevision
	for _, rev := range revs {
		if rev.GetName() == p.GetCurrentRevision() {
			current = rev
			continue
		}

		rev.SetDesiredState(v1.PackageRevisionInactive)
		if err := a.client.Apply(ctx, rev, resource.MustBeControllableBy(p.GetUID())); err != nil {
			return nil, errors.Wrap(err, errUpdateInactivePackageRevision)
		}
	}

	current.SetDesiredState(v1.PackageRevisionActive)
	if err := a.client.Apply(ctx, current, resource.MustBeControllableBy(p.GetUID())); err != nil {
		return nil, errors.Wrap(err, errUpdateActivePackageRevision)
	}

	return revs, nil
}

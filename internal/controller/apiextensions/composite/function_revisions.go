/*
Copyright 2026 The Crossplane Authors.

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

package composite

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

const (
	errGetFunction           = "cannot get Function"
	errListFunctionRevisions = "cannot list FunctionRevisions"

	errFmtNoActiveFunctionRevision = "Function %q has no active FunctionRevision"
)

// FunctionRevisionForStep returns the name of the FunctionRevision that should
// run the supplied Composition pipeline step.
func FunctionRevisionForStep(ctx context.Context, c client.Reader, s v1.PipelineStep) (string, error) {
	return ActiveFunctionRevision(ctx, c, s.FunctionRef.Name)
}

// ActiveFunctionRevision returns the name of the active FunctionRevision of
// the named Function. Only revisions the Function controls - i.e. revisions
// managed by the package manager - are considered.
func ActiveFunctionRevision(ctx context.Context, c client.Reader, fnName string) (string, error) {
	fn := &pkgv1.Function{}
	if err := c.Get(ctx, client.ObjectKey{Name: fnName}, fn); err != nil {
		return "", errors.Wrap(err, errGetFunction)
	}

	l := &pkgv1.FunctionRevisionList{}
	if err := c.List(ctx, l, client.MatchingLabels{pkgv1.LabelParentPackage: fnName}); err != nil {
		return "", errors.Wrap(err, errListFunctionRevisions)
	}

	var active *pkgv1.FunctionRevision
	for i := range l.Items {
		rev := &l.Items[i]
		if rev.GetDesiredState() != pkgv1.PackageRevisionActive {
			continue
		}

		if !metav1.IsControlledBy(rev, fn) {
			continue
		}

		// The package manager only activates one revision at a time, but
		// prefer the newest just in case, so the result is deterministic.
		if active == nil || rev.GetRevision() > active.GetRevision() {
			active = rev
		}
	}

	if active == nil {
		return "", errors.Errorf(errFmtNoActiveFunctionRevision, fnName)
	}

	return active.GetName(), nil
}

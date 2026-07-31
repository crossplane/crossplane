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
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/go-containerregistry/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

const (
	errGetFunction           = "cannot get Function"
	errGetFunctionRevision   = "cannot get FunctionRevision"
	errListFunctionRevisions = "cannot list FunctionRevisions"

	errFmtNoActiveFunctionRevision   = "Function %q has no active FunctionRevision"
	errFmtNoExternalFunctionRevision = "no active FunctionRevision exists for function package %q"
	errFmtParseFunctionPackage       = "cannot parse function package %q; it must be a fully qualified OCI reference specified by digest"
)

// FunctionRevisionForStep returns the name of the FunctionRevision that should
// run the supplied Composition pipeline step.
func FunctionRevisionForStep(ctx context.Context, c client.Reader, s v1.PipelineStep) (string, error) {
	if s.Function != "" {
		return ExternalFunctionRevision(ctx, c, s.Function)
	}

	if s.FunctionRef == nil {
		return "", errors.Errorf("pipeline step %s is invalid: missing both function and functionRef", s.Step)
	}

	return ActiveFunctionRevision(ctx, c, s.FunctionRef.Name)
}

// ActiveFunctionRevision returns the name of the active FunctionRevision of the
// named Function. Only revisions the Function controls - i.e. revisions managed
// by the package manager - are considered.
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

// ExternalFunctionRevision returns the name of the active, externally managed
// FunctionRevision for the supplied function package OCI reference.
func ExternalFunctionRevision(ctx context.Context, c client.Reader, pkg string) (string, error) {
	ref, err := NormalizeFunctionOCIRef(pkg)
	if err != nil {
		return "", err
	}

	revName := FunctionRevisionName(ref)

	var rev pkgv1.FunctionRevision
	if err := c.Get(ctx, types.NamespacedName{Name: revName}, &rev); err != nil {
		return "", errors.Wrap(err, errGetFunctionRevision)
	}

	if rev.GetDesiredState() != pkgv1.PackageRevisionActive {
		return "", errors.Errorf(errFmtNoExternalFunctionRevision, pkg)
	}

	got, err := NormalizeFunctionOCIRef(rev.Spec.Package)
	if err != nil {
		return "", errors.Wrapf(err, "function revision %q has invalid package ref", rev.Name)
	}
	if got.Name() != ref.Name() {
		return "", errors.Errorf(errFmtNoExternalFunctionRevision, pkg)
	}

	return rev.Name, nil
}

// NormalizeFunctionOCIRef returns ref with the tag omitted, if it had one. It
// is an error to pass an OCI ref that is not fully qualified and by digest.
func NormalizeFunctionOCIRef(ref string) (name.Digest, error) {
	d, err := name.NewDigest(ref, name.StrictValidation)
	if err != nil {
		return name.Digest{}, errors.Wrapf(err, errFmtParseFunctionPackage, ref)
	}

	return d.Context().Digest(d.DigestStr()), nil
}

// FunctionName derives a Function name from a package reference. Names are
// constructed using the same method the package manager uses when installing
// dependencies, so we may share functions with the package manager.
func FunctionName(ref name.Digest) string {
	return xpkg.ToDNSLabel(ref.Context().RepositoryStr())
}

// FunctionRevisionName derives a FunctionRevision name from a package
// reference. Names are based on the full OCI reference, so that multiple
// composition revisions referencing the same package will share a function
// revision. Note, however, that we construct names differently from the package
// manager, so we will not share revisions with it.
func FunctionRevisionName(ref name.Digest) string {
	fnName := FunctionName(ref)
	h := sha256.Sum256([]byte(ref.Name()))
	return xpkg.FriendlyID(fnName, hex.EncodeToString(h[:]))
}

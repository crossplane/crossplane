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

package transaction

import (
	"context"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/xcrd"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

// ValidatorChain runs multiple validators in sequence (fail-fast).
type ValidatorChain []Validator

// Validate runs all validators in the chain, stopping at the first error.
func (c ValidatorChain) Validate(ctx context.Context, tx *v1alpha1.Transaction) error {
	for _, v := range c {
		if err := v.Validate(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// SchemaValidator validates CRD schema compatibility.
type SchemaValidator struct {
	kube client.Client
	pkg  xpkg.Client
}

// NewSchemaValidator returns a new SchemaValidator.
func NewSchemaValidator(kc client.Client, pc xpkg.Client) *SchemaValidator {
	return &SchemaValidator{kube: kc, pkg: pc}
}

// Validate checks that proposed package CRDs don't introduce breaking schema changes.
func (v *SchemaValidator) Validate(ctx context.Context, tx *v1alpha1.Transaction) error {
	for _, lockPkg := range tx.Status.ProposedLockPackages {
		ref := xpkg.BuildReference(lockPkg.Source, lockPkg.Version)

		pkg, err := v.pkg.Get(ctx, ref)
		if err != nil {
			return errors.Wrapf(err, "cannot fetch package %s", ref)
		}

		for _, obj := range pkg.GetObjects() {
			var crd *extv1.CustomResourceDefinition

			switch o := obj.(type) {
			case *extv1.CustomResourceDefinition:
				crd = o
			case *v1.CompositeResourceDefinition:
				crd, err = xcrd.ForCompositeResource(o)
				if err != nil {
					return errors.Wrapf(err, "cannot convert XRD %s to CRD", o.GetName())
				}
			default:
				continue
			}

			existing := &extv1.CustomResourceDefinition{}
			err := v.kube.Get(ctx, types.NamespacedName{Name: crd.Name}, existing)
			if kerrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return errors.Wrapf(err, "cannot get existing CRD %s", crd.Name)
			}

			results := xcrd.CompareSchemas(existing, crd, xcrd.WithAlphaExemption())
			if err := xcrd.BreakingChangesError(results...); err != nil {
				return errors.Wrapf(err, "package %s version %s contains breaking changes to CRD %s", lockPkg.Source, lockPkg.Version, crd.Name)
			}
		}
	}

	return nil
}

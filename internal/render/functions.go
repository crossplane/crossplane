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

package render

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// A FunctionRevisionRunner runs a function by FunctionRevision name, using a
// runner that's keyed by FunctionInput name. Reconcilers being rendered resolve
// pipeline steps to FunctionRevisions - either the synthetic revisions
// SyntheticFunctions returns, or revisions they created themselves. This runner
// maps each revision back to the FunctionInput that satisfies it.
type FunctionRevisionRunner struct {
	wrapped xfn.FunctionRunner
	client  client.Reader
	inputs  map[string]bool
}

// NewFunctionRevisionRunner returns a FunctionRevisionRunner that runs
// functions using the supplied runner, which must be keyed by the supplied
// FunctionInputs' names. FunctionRevisions are read using the supplied client.
func NewFunctionRevisionRunner(wrapped xfn.FunctionRunner, c client.Reader, fns []*renderv1alpha1.FunctionInput) *FunctionRevisionRunner {
	inputs := make(map[string]bool, len(fns))
	for _, fn := range fns {
		inputs[fn.GetName()] = true
	}

	return &FunctionRevisionRunner{wrapped: wrapped, client: c, inputs: inputs}
}

// RunFunction runs the FunctionInput that satisfies the named FunctionRevision.
// That's the FunctionInput named after the revision's package OCI reference,
// or named after the revision's parent Function.
func (r *FunctionRevisionRunner) RunFunction(ctx context.Context, rev string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	fr := &pkgv1.FunctionRevision{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: rev}, fr); err != nil {
		return nil, errors.Wrapf(err, "cannot get FunctionRevision %q", rev)
	}

	for _, name := range []string{fr.Spec.Package, fr.GetLabels()[pkgv1.LabelParentPackage]} {
		if r.inputs[name] {
			return r.wrapped.RunFunction(ctx, name, req)
		}
	}

	return nil, errors.Errorf("no function in the render input satisfies FunctionRevision %q (package %q)", rev, fr.Spec.Package)
}

// SyntheticFunctions returns synthetic Function and FunctionRevision resources
// for the supplied render function inputs. They let the reconcilers being
// rendered resolve each pipeline step to a FunctionRevision as they would in a
// real cluster. Use a FunctionRevisionRunner to map the FunctionRevisions back
// to their FunctionInputs.
//
// A FunctionInput named with a digest OCI reference satisfies pipeline steps
// that reference a function by that OCI reference, so it gets an external
// FunctionRevision with that package, named as the reconcilers being rendered
// would name it. Any other FunctionInput satisfies
// pipeline steps that reference a Function by name, so it gets a Function with
// an active FunctionRevision that it controls.
func SyntheticFunctions(fns []*renderv1alpha1.FunctionInput) ([]kunstructured.Unstructured, error) {
	out := make([]kunstructured.Unstructured, 0, 2*len(fns))

	for _, fn := range fns {
		rev := &pkgv1.FunctionRevision{}
		rev.SetName(fn.GetName())
		rev.Spec.DesiredState = pkgv1.PackageRevisionActive

		if ref, err := xcomposite.NormalizeFunctionOCIRef(fn.GetName()); err == nil {
			rev.SetName(xcomposite.FunctionRevisionName(ref))
			rev.SetLabels(map[string]string{pkgv1.LabelParentPackage: xcomposite.FunctionName(ref)})
			rev.Spec.Package = fn.GetName()

			u, err := syntheticUnstructured(rev, pkgv1.FunctionRevisionGroupVersionKind)
			if err != nil {
				return nil, err
			}

			out = append(out, u)

			continue
		}

		f := &pkgv1.Function{}
		f.SetName(fn.GetName())
		f.SetUID(types.UID("render-" + fn.GetName()))

		rev.SetLabels(map[string]string{pkgv1.LabelParentPackage: f.GetName()})
		rev.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: pkgv1.FunctionGroupVersionKind.GroupVersion().String(),
			Kind:       pkgv1.FunctionKind,
			Name:       f.GetName(),
			UID:        f.GetUID(),
			Controller: new(true),
		}})

		fu, err := syntheticUnstructured(f, pkgv1.FunctionGroupVersionKind)
		if err != nil {
			return nil, err
		}

		ru, err := syntheticUnstructured(rev, pkgv1.FunctionRevisionGroupVersionKind)
		if err != nil {
			return nil, err
		}

		out = append(out, fu, ru)
	}

	return out, nil
}

func syntheticUnstructured(o runtime.Object, gvk schema.GroupVersionKind) (kunstructured.Unstructured, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return kunstructured.Unstructured{}, errors.Wrapf(err, "cannot convert %s to unstructured", gvk.Kind)
	}

	u := kunstructured.Unstructured{Object: m}
	u.SetGroupVersionKind(gvk)

	return u, nil
}

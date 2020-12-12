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

package xpkg

import (
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

const (
	errNotConvertible = "supplied object was not convertible"
	errNoConversions  = "supplied object could not be converted to any of the supplied candidates"
)

// BuildMetaScheme builds the default scheme used for identifying metadata in a
// Crossplane package.
func BuildMetaScheme() (*runtime.Scheme, error) {
	metaScheme := runtime.NewScheme()
	if err := pkgmetav1alpha1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	if err := pkgmetav1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	return metaScheme, nil
}

// BuildObjectScheme builds the default scheme used for identifying objects in a
// Crossplane package.
func BuildObjectScheme() (*runtime.Scheme, error) {
	objScheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := v1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := extv1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := extv1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	return objScheme, nil
}

// ConvertTo converts the supplied object to the first supplied candidate that
// does not return an error.
func ConvertTo(obj runtime.Object, candidates ...conversion.Hub) (runtime.Object, error) {
	cvt, ok := obj.(conversion.Convertible)
	if !ok {
		return nil, errors.New(errNotConvertible)
	}

	for _, c := range candidates {
		c := c
		if err := cvt.ConvertTo(c); err == nil {
			return c, nil
		}
	}

	return nil, errors.New(errNoConversions)
}

// ConvertToPkg converts the supplied object to a pkgmeta.Pkg, if possible.
func ConvertToPkg(obj runtime.Object, candidates ...conversion.Hub) (pkgmeta.Pkg, bool) {
	po, err := ConvertTo(obj, candidates...)
	if err != nil {
		return nil, false
	}
	m, ok := po.(pkgmeta.Pkg)
	return m, ok
}

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
	admv1 "k8s.io/api/admissionregistration/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
)

// BuildMetaScheme builds the default scheme used for identifying metadata in a
// Crossplane package.
func BuildMetaScheme() (*runtime.Scheme, error) {
	metaScheme := runtime.NewScheme()
	if err := pkgmetav1alpha1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		return nil, err
	}

	if err := pkgmetav1beta1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
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
	if err := v1.AddToScheme(objScheme); err != nil {
		return nil, err
	}

	if err := v2.AddToScheme(objScheme); err != nil {
		return nil, err
	}

	if err := extv1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}

	if err := extv1.AddToScheme(objScheme); err != nil {
		return nil, err
	}

	if err := admv1.AddToScheme(objScheme); err != nil {
		return nil, err
	}

	return objScheme, nil
}

// TryConvert converts the supplied object to the first supplied candidate that
// does not return an error. Returns the converted object and true when
// conversion succeeds, or the original object and false if it does not.
func TryConvert(obj runtime.Object, candidates ...conversion.Hub) (runtime.Object, bool) {
	// Note that if we already converted the supplied object to one of the
	// supplied Hubs in a previous call this will ensure we skip conversion if
	// and when it's called again.
	cvt, ok := obj.(conversion.Convertible)
	if !ok {
		return obj, false
	}

	for _, c := range candidates {
		if err := cvt.ConvertTo(c); err == nil {
			return c, true
		}
	}

	return obj, false
}

// TryConvertToPkg converts the supplied object to a pkgmeta.Pkg, if possible.
func TryConvertToPkg(obj runtime.Object, candidates ...conversion.Hub) (pkgmetav1.Pkg, bool) {
	po, _ := TryConvert(obj, candidates...)
	m, ok := po.(pkgmetav1.Pkg)

	return m, ok
}

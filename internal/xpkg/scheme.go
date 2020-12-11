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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
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

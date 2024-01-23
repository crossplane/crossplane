/*
Copyright 2021 The Crossplane Authors.

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

// Package controller contains options specific to apiextensions controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.com/crossplane/crossplane/internal/xfn"
)

// Options specific to apiextensions controllers.
type Options struct {
	controller.Options

	// Namespace in which we'll look for image pull secrets for in-cluster
	// private registry authentication when pulling Composition Functions.
	Namespace string

	// ServiceAccount for which we'll find image pull secrets for in-cluster
	// private registry authentication when pulling Composition Functions.
	ServiceAccount string

	// FunctionRunner used to run Composition Functions.
	FunctionRunner *xfn.PackagedFunctionRunner
}

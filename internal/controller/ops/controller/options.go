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

// Package controller contains options specific to ops controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.com/crossplane/crossplane/internal/engine"
	"github.com/crossplane/crossplane/internal/xfn"
)

// Options specific to apiextensions controllers.
type Options struct {
	controller.Options

	// FunctionRunner used to run Composition Functions.
	FunctionRunner xfn.FunctionRunner

	// ControllerEngine used to dynamically manage watches.
	ControllerEngine *engine.ControllerEngine
}

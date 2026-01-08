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

package xpkg

import "github.com/crossplane/crossplane-runtime/v2/pkg/parser"

// Validator validates packages before installation is attempted.
type Validator parser.Linter

// NewProviderValidator is a convenience function for creating a package
// validator for providers.
func NewProviderValidator() Validator {
	return parser.NewPackageLinter(
		parser.PackageLinterFns(OneMeta),
		parser.ObjectLinterFns(IsProvider, PackageValidSemver),
		parser.ObjectLinterFns())
}

// NewConfigurationValidator is a convenience function for creating a package
// validator for configurations.
func NewConfigurationValidator() Validator {
	return parser.NewPackageLinter(
		parser.PackageLinterFns(OneMeta),
		parser.ObjectLinterFns(IsConfiguration, PackageValidSemver),
		parser.ObjectLinterFns())
}

// NewFunctionValidator is a convenience function for creating a package
// validator for functions.
func NewFunctionValidator() Validator {
	return parser.NewPackageLinter(
		parser.PackageLinterFns(OneMeta),
		parser.ObjectLinterFns(IsFunction, PackageValidSemver),
		parser.ObjectLinterFns())
}


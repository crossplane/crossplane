// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xpkg

import (
	"context"
	"io"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/ndjson"
)

// JSONPackageParser defines the API contract for working with a
// PackageParser.
type JSONPackageParser interface {
	Parse(context.Context, io.ReadCloser) (*ndjson.Package, error)
}

// ParsedPackage represents an xpkg that has been parsed from a v1.Image
type ParsedPackage struct {
	// The package dependencies derived from .Spec.DependsOn.
	Deps []v1beta1.Dependency
	// The MetaObj file that corresponds to the package.
	MetaObj runtime.Object
	// The name of the package. This name maps to the package name defined
	// in the crossplane.yaml and is represented in the directory name for
	// the package on the filesystem.
	DepName string
	// The N corresponding Objs (CRDs, XRDs, Compositions) depending on the package type.
	Objs []runtime.Object
	// The type of Package.
	PType v1beta1.PackageType
	// The container registry.
	Reg string
	// The SHA corresponding to the package.
	SHA string
	// The resolved version, e.g. v0.20.0
	Ver string
}

// Digest returns the package's digest derived from the package image.
func (p *ParsedPackage) Digest() string {
	return p.SHA
}

// Dependencies returns the package's dependencies.
func (p *ParsedPackage) Dependencies() []v1beta1.Dependency {
	return p.Deps
}

// Meta returns the runtime.Object corresponding to the meta file.
func (p *ParsedPackage) Meta() runtime.Object {
	return p.MetaObj
}

// Name returns the name of the package. e.g. crossplane/provider-aws.
func (p *ParsedPackage) Name() string {
	return p.DepName
}

// Objects returns the slice of runtime.Objects corresponding to CRDs, XRDs, and
// Compositions contained in the package.
func (p *ParsedPackage) Objects() []runtime.Object {
	return p.Objs
}

// Type returns the package's type.
func (p *ParsedPackage) Type() v1beta1.PackageType {
	return p.PType
}

// Registry returns the registry path where the package image is located.
// e.g. index.docker.io/crossplane/provider-aws
func (p *ParsedPackage) Registry() string {
	return p.Reg
}

// Version returns the version for the package image.
// e.g. v0.20.0
func (p *ParsedPackage) Version() string {
	return p.Ver
}

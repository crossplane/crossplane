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

package controller

import (
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ActiveRuntime is the runtime to use for packages with runtime.
type ActiveRuntime struct {
	runtimes       map[PackageKind]PackageRuntime
	defaultRuntime PackageRuntime
}

// RuntimeOption defines options to build up an active runtime.
type RuntimeOption func(runtime *ActiveRuntime)

// WithDefaultPackageRuntime marks a default runtime for unset kinds.
func WithDefaultPackageRuntime(runtime PackageRuntime) RuntimeOption {
	return func(ar *ActiveRuntime) {
		ar.defaultRuntime = runtime
	}
}

// WithPackageRuntime associates a runtime to a kind.
func WithPackageRuntime(kind PackageKind, runtime PackageRuntime) RuntimeOption {
	return func(ar *ActiveRuntime) {
		ar.runtimes[kind] = runtime
	}
}

// NewActiveRuntime builds an ActiveRuntime based on the provided options.
func NewActiveRuntime(o ...RuntimeOption) ActiveRuntime {
	r := ActiveRuntime{
		runtimes: make(map[PackageKind]PackageRuntime),
	}
	for _, o := range o {
		o(&r)
	}

	if r.defaultRuntime == PackageRuntimeUnspecified {
		r.defaultRuntime = PackageRuntimeDeployment
	}

	return r
}

// For returns the associated runtime for a given kind.
func (r ActiveRuntime) For(kind string) PackageRuntime {
	if runtime, ok := r.runtimes[PackageKind(kind)]; ok {
		return runtime
	}

	return r.defaultRuntime
}

// PackageRuntime is the runtime to use for packages with runtime.
type PackageRuntime string

// PackageKind is the name of the package kind.
type PackageKind string

const (
	// PackageRuntimeUnspecified means no package runtime is specified.
	PackageRuntimeUnspecified PackageRuntime = ""
	// PackageRuntimeDeployment uses a Kubernetes Deployment as the package
	// runtime.
	PackageRuntimeDeployment PackageRuntime = "Deployment"
	// PackageRuntimeExternal defer package runtime to an external controller.
	PackageRuntimeExternal PackageRuntime = "External"

	// ConfigurationPackageKind is for [Configuration,ConfigurationRevision].pkg.crossplane.io.
	ConfigurationPackageKind PackageKind = "Configuration"
	// ProviderPackageKind is for [Provider,ProviderRevision].pkg.crossplane.io.
	ProviderPackageKind PackageKind = "Provider"
	// FunctionPackageKind is for [Function,FunctionRevision].pkg.crossplane.io.
	FunctionPackageKind PackageKind = "Function"
)

// ParsePackageRuntime takes in a free form package runtime option string and attempts to parse it into an ActiveRuntime.
// Example input: 'Provider=Deployment;Function=External'.
// Note, This input matches the kong cli parsing for a map type input.
func ParsePackageRuntime(input string) (ActiveRuntime, error) {
	var opts []RuntimeOption

	for _, pkgRt := range strings.Split(input, ";") {
		parts := strings.Split(pkgRt, "=")
		if len(parts) != 2 {
			return ActiveRuntime{}, errors.Errorf("invalid package runtime setting %q, expected: runtime=kind", pkgRt)
		}

		pkg := PackageKind(strings.TrimSpace(parts[0]))
		rt := PackageRuntime(strings.TrimSpace(parts[1]))

		switch pkg {
		case ConfigurationPackageKind, ProviderPackageKind, FunctionPackageKind:
			switch rt {
			case PackageRuntimeDeployment, PackageRuntimeExternal:
				opts = append(opts, WithPackageRuntime(pkg, rt))
			case PackageRuntimeUnspecified:
				fallthrough
			default:
				return ActiveRuntime{}, errors.Errorf("unknown package runtime %q, supported: [%s]", rt,
					strings.Join([]string{string(PackageRuntimeDeployment), string(PackageRuntimeExternal)}, ", "))
			}
		default:
			return ActiveRuntime{}, errors.Errorf("unknown package runtime kind %q, supported: [%s]", pkg,
				strings.Join([]string{string(ConfigurationPackageKind), string(ProviderPackageKind), string(FunctionPackageKind)}, ", "))
		}
	}

	return NewActiveRuntime(opts...), nil
}

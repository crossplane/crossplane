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

package initializer

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errListProviders      = "failed getting provider list"
	errListConfigurations = "failed getting configuration list"
	errListFunctions      = "failed getting function list"
	errParsePackageName   = "package name is not valid"
	errApplyPackage       = "cannot apply package"
)

// NewPackageInstaller returns a new package installer.
func NewPackageInstaller(p []string, c []string, f []string) *PackageInstaller {
	return &PackageInstaller{
		providers:      p,
		configurations: c,
		functions:      f,
	}
}

// PackageInstaller has the initializer for installing a list of packages.
type PackageInstaller struct {
	configurations []string
	providers      []string
	functions      []string
}

// Run makes sure all specified packages exist.
func (pi *PackageInstaller) Run(ctx context.Context, kube client.Client) error {
	pkgs := make([]client.Object, len(pi.providers)+len(pi.configurations)+len(pi.functions))
	// NOTE(hasheddan): we build maps of existing Provider, Configuration and Function
	// sources to the package names such that we can update the version when a
	// package specified for install matches the source of an existing package.
	pl := &v1.ProviderList{}
	if err := kube.List(ctx, pl); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, errListProviders)
	}
	pMap := make(map[string]string, len(pl.Items))
	for _, p := range pl.Items {
		ref, err := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(""))
		if err != nil {
			// NOTE(hasheddan): we skip package sources that are not have valid
			// references because we cannot make assumptions about their
			// versioning. The only case in which a package source can be an
			// invalid reference is if it was preloaded into the package cache
			// and its packagePullPolicy is set to Never.
			continue
		}
		pMap[xpkg.ParsePackageSourceFromReference(ref)] = p.GetName()
	}
	cl := &v1.ConfigurationList{}
	if err := kube.List(ctx, cl); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, errListConfigurations)
	}
	cMap := make(map[string]string, len(cl.Items))
	for _, c := range cl.Items {
		ref, err := name.ParseReference(c.GetSource(), name.WithDefaultRegistry(""))
		if err != nil {
			continue
		}
		cMap[xpkg.ParsePackageSourceFromReference(ref)] = c.GetName()
	}
	fl := &v1.FunctionList{}
	if err := kube.List(ctx, fl); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, errListFunctions)
	}
	fMap := make(map[string]string, len(fl.Items))
	for _, f := range fl.Items {
		ref, err := name.ParseReference(f.GetSource(), name.WithDefaultRegistry(""))
		if err != nil {
			continue
		}
		fMap[xpkg.ParsePackageSourceFromReference(ref)] = f.GetName()
	}
	// NOTE(hasheddan): we maintain a separate index from the range so that
	// Providers, Configurations and Functions can be added to the same slice for applying.
	pkgsIdx := 0
	for _, img := range pi.providers {
		p := &v1.Provider{}
		if err := buildPack(p, img, pMap); err != nil {
			return err
		}
		pkgs[pkgsIdx] = p
		pkgsIdx++
	}
	for _, img := range pi.configurations {
		c := &v1.Configuration{}
		if err := buildPack(c, img, cMap); err != nil {
			return err
		}
		pkgs[pkgsIdx] = c
		pkgsIdx++
	}
	for _, img := range pi.functions {
		f := &v1.Function{}
		if err := buildPack(f, img, fMap); err != nil {
			return err
		}
		pkgs[pkgsIdx] = f
		pkgsIdx++
	}
	pa := resource.NewAPIPatchingApplicator(kube)
	for _, p := range pkgs {
		if err := pa.Apply(ctx, p); err != nil {
			return errors.Wrap(err, errApplyPackage)
		}
	}
	return nil
}

func buildPack(pack v1.Package, img string, pkgMap map[string]string) error {
	ref, err := name.ParseReference(img, name.WithDefaultRegistry(""))
	if err != nil {
		return errors.Wrap(err, errParsePackageName)
	}
	objName := xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	if existing, ok := pkgMap[ref.Context().RepositoryStr()]; ok {
		objName = existing
	}
	pack.SetName(objName)
	pack.SetSource(ref.String())
	return nil
}

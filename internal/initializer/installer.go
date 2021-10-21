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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errGetLock          = "failed getting lock object"
	errParsePackageName = "package name is not valid"
	errApplyPackage     = "cannot apply package"
)

// NewPackageInstaller returns a new package installer.
func NewPackageInstaller(p []string, c []string) *PackageInstaller {
	return &PackageInstaller{
		providers:      p,
		configurations: c,
	}
}

// PackageInstaller has the initializer for installing a list of packages.
type PackageInstaller struct {
	configurations []string
	providers      []string
}

// Run makes sure all specified packages exist.
// NOTE(hasheddan): this function is over our cyclomatic complexity goal, but
// only runs at installation time and is performing fairly straightforward
// operations.
func (pi *PackageInstaller) Run(ctx context.Context, kube client.Client) error { //nolint:gocyclo
	pkgs := make([]client.Object, len(pi.providers)+len(pi.configurations))
	// NOTE(hasheddan): we build a map of existing installed package sources to
	// their Provider or Configuration names so that we can update the source
	// without creating a new package install.
	l := &v1beta1.Lock{}
	if err := kube.Get(ctx, types.NamespacedName{Name: "lock"}, l); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, errGetLock)
	}
	pkgMap := make(map[string]string)
	if l != nil {
		for _, p := range l.Packages {
			pkgMap[p.Source] = p.Name
		}
	}
	// NOTE(hasheddan): we maintain a separate index from the range so that
	// Providers and Configurations can be added to the same slice for applying.
	pkgsIdx := 0
	for _, img := range pi.providers {
		p := &v1.Provider{}
		if err := buildPack(p, img, pkgMap); err != nil {
			return err
		}
		pkgs[pkgsIdx] = p
		pkgsIdx++
	}
	for _, img := range pi.configurations {
		c := &v1.Configuration{}
		if err := buildPack(c, img, pkgMap); err != nil {
			return err
		}
		pkgs[pkgsIdx] = c
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

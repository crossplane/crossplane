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

package pkg

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	crds "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/apis/apiextensions"
	"github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

// Cmd is the root package command.
type Cmd struct {
	Build BuildCmd `cmd:"" help:"Build a Crossplane package."`
	Push  PushCmd  `cmd:"" help:"Push a Crossplane package to a registry."`
	Print PrintCmd `cmd:"" help:"Print the output of a Crossplane package."`
}

// Run runs the package command.
func (r *Cmd) Run() error {
	return nil
}

// BuildCmd build a package.
type BuildCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of the package to be built. Defaults to name in crossplane.yaml."`

	Tag      string `short:"t" help:"Package version tag." default:"latest"`
	Registry string `short:"r" help:"Package registry." default:"registry.upbound.io"`
}

// Run runs the Build command.
func (b *BuildCmd) Run() error {
	return nil
}

// PushCmd pushes a package to a registry.
type PushCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of the package to be pushed. Defaults to name in crossplane.yaml."`

	Tag      string `short:"t" help:"Package version tag." default:"latest"`
	Registry string `short:"r" help:"Package registry." default:"registry.upbound.io"`
}

// Run runs the Push command.
func (p *PushCmd) Run() error {
	return nil
}

// PrintCmd prints the output of the package in the directory.
type PrintCmd struct {
	Path string `name:"path" short:"p" type:"path" help:"The path of the package in the local file system. It is the directory where crossplane.yaml exists." default:"."`
}

// Run runs the Push command.
func (c *PrintCmd) Run() error {
	ctx := context.Background()
	metaScheme, err := v1alpha1.SchemeBuilder.Build()
	if err != nil {
		return err
	}
	objScheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		apiextensions.AddToScheme,
		crds.AddToScheme,
	} {
		if err := add(objScheme); err != nil {
			return err
		}
	}
	p := parser.New(metaScheme, objScheme)

	b := parser.NewFsBackend(afero.NewReadOnlyFs(afero.NewOsFs()), parser.FsDir(c.Path), parser.FsFilters(parser.SkipNotYAML()))
	reader, err := b.Init(ctx)
	if err != nil {
		return err
	}
	pkg, err := p.Parse(ctx, reader)
	if err != nil {
		return err
	}
	list := append(pkg.GetMeta(), pkg.GetObjects()...)
	for _, m := range list {
		if m.GetObjectKind().GroupVersionKind().Empty() {
			continue
		}
		out, err := yaml.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "cannot marshall meta object into yaml")
		}
		// Leaving the new line character to the OS instead of one fmt.Printf.
		fmt.Println("---")
		fmt.Print(string(out))
	}
	return nil
}

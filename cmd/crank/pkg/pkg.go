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

// Cmd is the root package command.
type Cmd struct {
	Build BuildCmd `cmd:"" help:"Build a Crossplane package."`
	Push  PushCmd  `cmd:"" help:"Push a Crossplane package to a registry."`
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

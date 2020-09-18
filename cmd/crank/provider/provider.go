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

package provider

import (
	"context"

	"github.com/docker/docker/client"

	"github.com/crossplane/crossplane/cmd/crank/pkg"
	"github.com/crossplane/crossplane/pkg/controller/pkg/revision"
)

// Cmd is the root command for Providers.
type Cmd struct {
	Build  BuildCmd    `cmd:"" help:"Build a Provider Package."`
	Lint   LintCmd     `cmd:"" help:"Lint the contents of a Provider Package."`
	Push   pkg.PushCmd `cmd:"" help:"Push a Provider Package."`
	Create CreateCmd   `cmd:"" help:"Create a Provider."`
	Get    GetCmd      `cmd:"" help:"Get installed Providers."`
}

// Run runs the Provider command.
func (r *Cmd) Run() error {
	return nil
}

// BuildCmd builds a Provider.
type BuildCmd struct {
	pkg.BuildCmd
}

// Run runs the provider build cmd.
func (pbc *BuildCmd) Run() error {
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	err = pbc.BuildCmd.Build(ctx, docker)
	if err != nil {
		return err
	}

	if pbc.NoLint {
		return nil
	}
	lc := LintCmd{
		pkg.LintCmd{
			Image:  pbc.FullImageName(),
			NoPull: pbc.NoPull,
		},
		ctx,
		docker,
	}
	return lc.Run()
}

// LintCmd lints the contents of a Provider Package.
type LintCmd struct {
	pkg.LintCmd
	ctx    context.Context
	docker *client.Client
}

// Run runs the provider lint cmd
func (pc *LintCmd) Run() error {
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	linter := revision.NewPackageLinter(
		revision.PackageLinterFns(revision.OneMeta),
		revision.ObjectLinterFns(revision.IsProvider),
		revision.ObjectLinterFns(revision.Or(revision.IsCRD, revision.IsComposition)))

	return pc.LintCmd.Lint(ctx, docker, linter)
}

// CreateCmd creates a Provider in the cluster.
type CreateCmd struct {
	Name    string `arg:"" name:"name" help:"Name of Provider."`
	Package string `arg:"" name:"package" help:"Image containing Provider package."`

	History  int  `help:"Revision history limit."`
	Activate bool `short:"a" help:"Enable automatic revision activation policy."`
}

// Run the CreateCmd.
func (p *CreateCmd) Run() error {
	return nil
}

// GetCmd gets one or more Providers in the cluster.
type GetCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of Provider."`

	Revisions bool `short:"r" help:"List revisions for each Provider."`
}

// Run the Get command.
func (b *GetCmd) Run() error {
	return nil
}

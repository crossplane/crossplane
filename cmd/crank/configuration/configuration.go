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

package configuration

import (
	"context"

	"github.com/docker/docker/client"

	"github.com/crossplane/crossplane/cmd/crank/pkg"
	"github.com/crossplane/crossplane/pkg/controller/pkg/revision"
)

// Cmd is the root command for Configurations.
type Cmd struct {
	Build  BuildCmd    `cmd:"" help:"Build a Configuration."`
	Lint   LintCmd     `cmd:"" help:"Lint the contents of a Configuration Package."`
	Push   pkg.PushCmd `cmd:"" help:"Push a Configuration Package."`
	Create CreateCmd   `cmd:"" help:"Create a Configuration."`
	Get    GetCmd      `cmd:"" help:"Get installed Configurations."`
}

// Run runs the Configuration command.
func (r *Cmd) Run() error {
	return nil
}

// BuildCmd builds a Configuration.
type BuildCmd struct {
	pkg.BuildCmd
}

// Run runs the configuration build cmd.
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

// LintCmd lints the contents of a Configuration Package.
type LintCmd struct {
	pkg.LintCmd
	ctx    context.Context
	docker *client.Client
}

// Run runs the configuration lint cmd
func (pc *LintCmd) Run() error {
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	linter := revision.NewPackageLinter(
		revision.PackageLinterFns(revision.OneMeta),
		revision.ObjectLinterFns(revision.IsConfiguration),
		revision.ObjectLinterFns(revision.IsXRD))
	return pc.LintCmd.Lint(ctx, docker, linter)
}

// CreateCmd creates a Configuration in the cluster.
type CreateCmd struct {
	Name    string `arg:"" name:"name" help:"Name of Configuration."`
	Package string `arg:"" name:"package" help:"Image containing Configuration package."`

	History  int  `help:"Revision history limit."`
	Activate bool `short:"a" help:"Enable automatic revision activation policy."`
}

// Run the CreateCmd.
func (p *CreateCmd) Run() error {
	return nil
}

// GetCmd gets one or more Configurations in the cluster.
type GetCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of Configuration."`

	Revisions bool `short:"r" help:"List revisions for each Configuration."`
}

// Run the Get command.
func (b *GetCmd) Run() error {
	return nil
}

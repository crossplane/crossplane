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

package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/internal/version"
)

var _ = kong.Must(&cli)

type versionFlag string
type verboseFlag bool

// Decode overrides the default string decoder to be a no-op.
func (v versionFlag) Decode(ctx *kong.DecodeContext) error { return nil } // nolint:unparam

// IsBool indicates that this string flag should be treated as a boolean value.
func (v versionFlag) IsBool() bool { return true }

// BeforeApply indicates that we want to execute the logic before running any
// commands.
func (v versionFlag) BeforeApply(app *kong.Kong) error { // nolint:unparam
	fmt.Fprintln(app.Stdout, version.New().GetVersionString())
	app.Exit(0)
	return nil
}

func (v verboseFlag) BeforeApply(ctx *kong.Context) error { // nolint:unparam
	logger := logging.NewLogrLogger(zap.New(zap.UseDevMode(true)))
	ctx.BindTo(logger, (*logging.Logger)(nil))
	return nil
}

var cli struct {
	Version versionFlag `short:"v" name:"version" help:"Print version and quit."`
	Verbose verboseFlag `name:"verbose" help:"Print verbose logging statements."`

	Build   buildCmd   `cmd:"" help:"Build Crossplane packages."`
	Install installCmd `cmd:"" help:"Install Crossplane packages."`
	Update  updateCmd  `cmd:"" help:"Update Crossplane packages."`
	Push    pushCmd    `cmd:"" help:"Push Crossplane packages."`
}

func main() {
	buildChild := &buildChild{
		fs: afero.NewOsFs(),
	}
	pushChild := &pushChild{
		fs: afero.NewOsFs(),
	}
	logger := logging.NewNopLogger()
	ctx := kong.Parse(&cli,
		kong.Name("kubectl crossplane"),
		kong.Description("A command line tool for interacting with Crossplane."),
		// Binding a variable to kong context makes it available to all commands
		// at runtime.
		kong.Bind(buildChild, pushChild),
		kong.BindTo(logger, (*logging.Logger)(nil)),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

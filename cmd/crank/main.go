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

// Package main implements Crossplane's crank CLI - aka crossplane CLI.
package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta"
	"github.com/crossplane/crossplane/cmd/crank/xpkg"
	"github.com/crossplane/crossplane/internal/version"
)

var _ = kong.Must(&cli{})

type (
	versionFlag string
	verboseFlag bool
)

// Decode overrides the default string decoder to be a no-op.
func (v versionFlag) Decode(_ *kong.DecodeContext) error { return nil }

// IsBool indicates that this string flag should be treated as a boolean value.
func (v versionFlag) IsBool() bool { return true }

// BeforeApply indicates that we want to execute the logic before running any
// commands.
func (v versionFlag) BeforeApply(app *kong.Kong) error { //nolint:unparam // BeforeApply requires this signature.
	fmt.Fprintln(app.Stdout, version.New().GetVersionString())
	app.Exit(0)
	return nil
}

func (v verboseFlag) BeforeApply(ctx *kong.Context) error { //nolint:unparam // BeforeApply requires this signature.
	logger := logging.NewLogrLogger(zap.New(zap.UseDevMode(true)))
	ctx.BindTo(logger, (*logging.Logger)(nil))
	return nil
}

// The top-level crossplane CLI.
type cli struct {
	// Subcommands and flags will appear in the CLI help output in the same
	// order they're specified here. Keep them in alphabetical order.

	// Subcommands.
	XPKG xpkg.Cmd `cmd:"" help:"Manage Crossplane packages."`

	// The alpha and beta subcommands are intentionally in a separate block. We
	// want them to appear after all other subcommands.
	Beta beta.Cmd `cmd:"" help:"Beta commands."`

	// Flags.
	Verbose verboseFlag `help:"Print verbose logging statements." name:"verbose"`
	Version versionFlag `help:"Print version and quit."           name:"version" short:"v"`
}

func main() {
	logger := logging.NewNopLogger()
	ctx := kong.Parse(&cli{},
		kong.Name("crossplane"),
		kong.Description("A command line tool for interacting with Crossplane."),
		// Binding a variable to kong context makes it available to all commands
		// at runtime.
		kong.BindTo(logger, (*logging.Logger)(nil)),
		kong.ConfigureHelp(kong.HelpOptions{
			FlagsLast:      true,
			Compact:        true,
			WrapUpperBound: 80,
		}),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

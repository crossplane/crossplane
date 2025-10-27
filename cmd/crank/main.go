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
	"os"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/willabides/kongplete"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/cmd/crank/alpha"
	"github.com/crossplane/crossplane/v2/cmd/crank/beta"
	"github.com/crossplane/crossplane/v2/cmd/crank/completion"
	"github.com/crossplane/crossplane/v2/cmd/crank/plugin"
	"github.com/crossplane/crossplane/v2/cmd/crank/render"
	"github.com/crossplane/crossplane/v2/cmd/crank/version"
	"github.com/crossplane/crossplane/v2/cmd/crank/xpkg"
)

var _ = kong.Must(&cli{})

type (
	verboseFlag bool
)

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
	Plugin plugin.Cmd  `cmd:"" help:"Manage crossplane CLI plugins."`
	Render render.Cmd  `cmd:"" help:"Render a composite resource (XR)."`
	XPKG   xpkg.Cmd    `cmd:"" help:"Manage Crossplane packages."`

	// The alpha and beta subcommands are intentionally in a separate block. We
	// want them to appear after all other subcommands.
	Alpha   alpha.Cmd   `cmd:"" help:"Alpha commands."`
	Beta    beta.Cmd    `cmd:"" help:"Beta commands."`
	Version version.Cmd `cmd:"" help:"Print the client and server version information for the current context."`

	// Flags.
	Verbose verboseFlag `help:"Print verbose logging statements." name:"verbose"`

	// Completion
	Completions kongplete.InstallCompletions `cmd:"" help:"Get shell (bash/zsh/fish) completions. You can source this command to get completions for the login shell. Example: 'source <(crossplane completions)'"`
}

func main() {
	// Check if this might be a plugin invocation
	if len(os.Args) > 1 {
		// Try to find and execute a plugin
		if err := tryPlugin(os.Args[1], os.Args[2:]); err == nil {
			// Plugin executed successfully
			return
		}
	}

	// Fall back to normal CLI parsing
	logger := logging.NewNopLogger()
	parser := kong.Must(&cli{},
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

	kongplete.Complete(parser,
		kongplete.WithPredictors(completion.Predictors()),
	)

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	err = ctx.Run()
	ctx.FatalIfErrorf(err)
}

// tryPlugin attempts to find and execute a plugin for the given command
func tryPlugin(cmd string, args []string) error {
	// Skip if it's a known built-in command or flag
	if isBuiltinCommand(cmd) {
		return fmt.Errorf("builtin command")
	}

	// Look for plugin
	pluginPath, err := plugin.FindPlugin(cmd)
	if err != nil {
		return err
	}

	// Execute plugin
	return plugin.Execute(pluginPath, args)
}

// isBuiltinCommand checks if the command is a known built-in command
func isBuiltinCommand(cmd string) bool {
	// Check for flags
	if strings.HasPrefix(cmd, "-") {
		return true
	}

	// Dynamically get built-in commands from the cli struct
	c := &cli{}
	t := reflect.TypeOf(*c)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check if field has a cmd tag (indicating it's a command)
		if cmdTag, ok := field.Tag.Lookup("cmd"); ok && cmdTag == "" {
			// Convert field name to lowercase command name
			// XPKG -> xpkg, Render -> render, Alpha -> alpha, etc.
			fieldName := strings.ToLower(field.Name)

			// Special case: Completions field name vs actual command
			if field.Name == "Completions" {
				fieldName = "completions"
			}

			if cmd == fieldName {
				return true
			}
		}
	}

	// Also check for common help aliases
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		return true
	}

	return false
}

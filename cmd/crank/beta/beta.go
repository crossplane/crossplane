// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package beta contains beta Crossplane CLI subcommands.
// These commands are experimental, and may be changed or removed in a future
// release.
package beta

import (
	"github.com/crossplane/crossplane/cmd/crank/beta/render"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace"
	"github.com/crossplane/crossplane/cmd/crank/beta/xpkg"
)

// Cmd contains beta commands.
type Cmd struct {
	// Subcommands and flags will appear in the CLI help output in the same
	// order they're specified here. Keep them in alphabetical order.
	Render render.Cmd `cmd:"" help:"Render a composite resource (XR)."`
	Trace  trace.Cmd  `cmd:"" help:"Trace a Crossplane resource to get a detailed output of its relationships, helpful for troubleshooting."`
	XPKG   xpkg.Cmd   `cmd:"" help:"Manage Crossplane packages."`
}

// Help output for crossplane beta.
func (c *Cmd) Help() string {
	return "WARNING: These commands may be changed or removed in a future release."
}

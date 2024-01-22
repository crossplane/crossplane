// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package xpkg contains Crossplane packaging commands.
package xpkg

// TODO(lsviben) add the rest of the commands from up (batch, xpextract).

// Cmd contains commands for interacting with xpkgs.
type Cmd struct {
	// Keep subcommands sorted alphabetically.
	Build   buildCmd   `cmd:"" help:"Build a new package."`
	Install installCmd `cmd:"" help:"Install a package in a control plane."`
	Login   loginCmd   `cmd:"" help:"Login to the default package registry."`
	Logout  logoutCmd  `cmd:"" help:"Logout of the default package registry."`
	Push    pushCmd    `cmd:"" help:"Push a package to a registry."`
	Update  updateCmd  `cmd:"" help:"Update a package in a control plane."`
}

// Help prints out the help for the xpkg command.
func (c *Cmd) Help() string {
	return `
Crossplane can be extended using packages. Crossplane packages are called xpkgs.
Crossplane supports configuration, provider and function packages. 

A package is an opinionated OCI image that contains everything needed to extend
a Crossplane control plane with new functionality. For example installing a
provider package extends Crossplane with support for new kinds of managed
resource (MR).

See https://docs.crossplane.io/latest/concepts/packages for more information.
`
}

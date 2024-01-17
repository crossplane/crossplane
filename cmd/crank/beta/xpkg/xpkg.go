// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package xpkg contains beta Crossplane packaging commands.
package xpkg

// Cmd contains commands for interacting with packages.
type Cmd struct {
	// Keep commands sorted alphabetically.
	Init initCmd `cmd:"" help:"Initialize a new package from a template."`
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

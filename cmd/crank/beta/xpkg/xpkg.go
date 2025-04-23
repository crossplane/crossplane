/*
Copyright 2025 The Crossplane Authors.

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

// Package xpkg contains Crossplane packaging commands.
package xpkg

// Cmd contains beta variant commands for interacting with xpkgs.
type Cmd struct {
	Append appendCmd `cmd:"" help:"Append package extensions to a remote package."`
}

// Help returns the help string for the xpkg command.
func (c *Cmd) Help() string {
	return `
Crossplane can be extended using packages. Crossplane packages are called xpkgs.
Crossplane supports configuration, provider and function packages.

This is the beta variant of the xpkg command group containing experimental
commands for interacting with xpkgs.

For the stable variant, use "crossplane xpkg --help".
`
}

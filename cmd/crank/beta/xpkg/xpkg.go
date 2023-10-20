/*
Copyright 2023 The Crossplane Authors.

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

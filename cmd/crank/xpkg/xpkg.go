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

// Package xpkg contains the Crossplane packaging commands.
package xpkg

// Cmd contains commands for interacting with xpkgs.
// TODO(lsviben) add the rest of the commands from up (batch, xpextract).
type Cmd struct {
	Build buildCmd `cmd:"" help:"Build a Crossplane package."`
	Push  pushCmd  `cmd:"" help:"Push a Crossplane package to a container registry."`
}

// Help prints out the help for the xpkg command.
func (c *Cmd) Help() string {
	return `
Read the Crossplane docs for information on packages.
  https://docs.crossplane.io/latest/concepts/packages/
`
}

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

// Package render implements alpha rendering commands.
package render

import (
	"github.com/crossplane/crossplane/v2/cmd/crank/alpha/render/op"
	"github.com/crossplane/crossplane/v2/cmd/crank/alpha/render/test"
	"github.com/crossplane/crossplane/v2/cmd/crank/alpha/render/xr"
)

// Cmd contains alpha render subcommands.
type Cmd struct {
	// Subcommands and flags will appear in the CLI help output in the same
	// order they're specified here. Keep them in alphabetical order.
	Op   op.Cmd   `cmd:"" help:"Render an operation."`
	Test test.Cmd `cmd:"" help:"Render composite resources (XRs) and assert results."`
	XR   xr.Cmd   `cmd:"" help:"Render a composite resource (XR)."`
}

// Help output for crossplane alpha render.
func (c *Cmd) Help() string {
	return `
Render Crossplane resources locally using functions.

These commands show you what resources Crossplane would create or mutate by
running function pipelines locally, without talking to a Crossplane control plane.
`
}

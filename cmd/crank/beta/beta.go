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

// Package beta contains beta Crossplane CLI subcommands.
// These commands are experimental, and may be changed or removed in a future
// release.
package beta

import (
	"github.com/crossplane/crossplane/cmd/crank/beta/convert"
	"github.com/crossplane/crossplane/cmd/crank/beta/render"
	"github.com/crossplane/crossplane/cmd/crank/beta/top"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/crossplane/crossplane/cmd/crank/beta/xpkg"
)

// Cmd contains beta commands.
type Cmd struct {
	// Subcommands and flags will appear in the CLI help output in the same
	// order they're specified here. Keep them in alphabetical order.
	Convert  convert.Cmd  `cmd:"" help:"Convert a Crossplane resource to a newer version or kind."`
	Render   render.Cmd   `cmd:"" help:"Render a composite resource (XR)."`
	Top      top.Cmd      `cmd:"" help:"Display resource (CPU/memory) usage by Crossplane related pods."`
	Trace    trace.Cmd    `cmd:"" help:"Trace a Crossplane resource to get a detailed output of its relationships, helpful for troubleshooting."`
	XPKG     xpkg.Cmd     `cmd:"" help:"Manage Crossplane packages."`
	Validate validate.Cmd `cmd:"" help:"Validate Crossplane resources."`
}

// Help output for crossplane beta.
func (c *Cmd) Help() string {
	return "WARNING: These commands may be changed or removed in a future release."
}

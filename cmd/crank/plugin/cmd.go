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

package plugin

// Cmd represents the plugin command group.
type Cmd struct {
	List ListCmd `cmd:"" help:"List all available plugins."`
}

// Help returns help instructions for the plugin command.
func (c *Cmd) Help() string {
	return `
Provides utilities for interacting with crossplane plugins.

Plugins provide extended functionality that is not part of the core crossplane distribution.
To install a plugin, place an executable binary starting with "crossplane-" anywhere in your PATH.

For example, a plugin binary named "crossplane-foo" will be available as:
  crossplane foo [args]

Examples:
  # List all available plugins
  crossplane plugin list

  # List plugin names only
  crossplane plugin list --name-only
`
}

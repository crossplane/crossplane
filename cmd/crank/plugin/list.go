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

import (
	"fmt"

	"github.com/alecthomas/kong"
)

// ListCmd represents the plugin list command.
type ListCmd struct {
	NameOnly bool `help:"Show only plugin names without full paths." name:"name-only"`
}

// Help returns help instructions for the plugin list command.
func (c *ListCmd) Help() string {
	return `
List all available crossplane plugins found in your PATH.

Plugins provide extended functionality that is not part of the core crossplane distribution.
Any executable in your PATH that starts with "crossplane-" will be detected as a plugin.

Examples:
  # List all available plugins with their full paths
  crossplane plugin list

  # List only plugin names
  crossplane plugin list --name-only
`
}

// Run executes the plugin list command.
func (c *ListCmd) Run(ctx *kong.Context) error {
	plugins, err := ListPlugins()
	if err != nil {
		return err
	}

	if len(plugins) == 0 {
		if _, err := fmt.Fprintln(ctx.Stdout, "No plugins found in PATH."); err != nil {
			return err
		}
		return nil
	}

	// Print header
	if !c.NameOnly {
		if _, err := fmt.Fprintln(ctx.Stdout, "The following plugins are available:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(ctx.Stdout); err != nil {
			return err
		}
	}

	// Print each plugin
	for _, plugin := range plugins {
		if c.NameOnly {
			if _, err := fmt.Fprintln(ctx.Stdout, plugin); err != nil {
				return err
			}
		} else {
			// Find full path for display
			pluginPath, err := FindPlugin(plugin)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(ctx.Stdout, "  %s\n    %s\n", plugin, pluginPath); err != nil {
				return err
			}
		}
	}

	return nil
}

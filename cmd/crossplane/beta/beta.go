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

// Package beta implements Crossplane beta (experimental) commands.
package beta

// Command defines the `crossplane beta` subcommand and its children.
type Command struct {
	Import ImportCommand `cmd:"" help:"Resource discovery and import (experimental)."`
}

// Run is a no-op for kong command tree.
// Kong requires each node in the calling path to have an associated Run method.
func (c *Command) Run() error {
	return nil
}

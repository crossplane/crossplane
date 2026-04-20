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

// Package upgrade contains commands that help users upgrade Crossplane.
package upgrade

import (
	"github.com/crossplane/crossplane/cmd/crank/beta/upgrade/check"
)

// Cmd groups Crossplane upgrade related commands.
type Cmd struct {
	Check check.Cmd `cmd:"" help:"Check a control plane for features that are removed or broken in Crossplane v2."`
}

// Help returns help for the upgrade command.
func (c *Cmd) Help() string {
	return "Commands associated with upgrading a Crossplane control plane to a newer version."
}

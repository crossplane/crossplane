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

// Package render implements the 'crossplane internal render' subcommand.
package render

import (
	"github.com/crossplane/crossplane/v2/cmd/crossplane/render/composite"
	"github.com/crossplane/crossplane/v2/cmd/crossplane/render/cronoperation"
	"github.com/crossplane/crossplane/v2/cmd/crossplane/render/operation"
	"github.com/crossplane/crossplane/v2/cmd/crossplane/render/watchoperation"
)

// Command routes to resource-specific render subcommands.
type Command struct {
	Composite      composite.Command      `cmd:""               help:"Render a composite resource using the real XR reconciler."`
	Operation      operation.Command      `cmd:""               help:"Render an operation using the real Operation reconciler."`
	CronOperation  cronoperation.Command  `cmd:"cronoperation"  help:"Produce the Operation a CronOperation would create."`
	WatchOperation watchoperation.Command `cmd:"watchoperation" help:"Produce the Operation a WatchOperation would create."`
}

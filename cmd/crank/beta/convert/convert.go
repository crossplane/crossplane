/*
Copyright 2024 The Crossplane Authors.

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

// Package convert contains Crossplane CLI subcommands for migrating Crossplane
// resources to newer versions or kinds.
package convert

import (
	"github.com/crossplane/crossplane/cmd/crank/beta/convert/compositionenvironment"
	"github.com/crossplane/crossplane/cmd/crank/beta/convert/deploymentruntime"
)

// Cmd converts a Crossplane resource to a newer version or a different kind.
type Cmd struct {
	DeploymentRuntime      deploymentruntime.Cmd      `cmd:"" help:"Convert a ControllerConfig to a DeploymentRuntimeConfig."`
	CompositionEnvironment compositionenvironment.Cmd `cmd:"" help:"Convert a Pipeline Composition to use function-environment-configs."`
}

// Help returns help message for the migrate command.
func (c *Cmd) Help() string {
	return `
This command converts a Crossplane resource to a newer version or a different kind.

Currently supported conversions:
* ControllerConfig -> DeploymentRuntimeConfig
* Classic Compositions -> Function Pipeline Compositions

Examples:
  # Write out a DeploymentRuntimeConfigFile from a ControllerConfig
  crossplane beta convert deployment-runtime cc.yaml -o drc.yaml

  # Convert an existing Composition to use Pipelines
  crossplane beta convert pipeline-composition composition.yaml -o pipeline-composition.yaml

  # Convert an existing Composition to use function-environment-configs instead of native Composition Environment,
  # requires the composition to be in Pipeline mode already.
  crossplane beta convert composition-environment composition.yaml -o composition-environment.yaml
`
}

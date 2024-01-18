package migrate

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/migrate/deploymentruntime"
	"github.com/crossplane/crossplane/cmd/crank/beta/migrate/pipelinecomposition"
)

// Cmd migrates a Crossplane resource to a newer version or a different kind.
type Cmd struct {
	DeploymentRuntime   deploymentruntime.Cmd   `cmd:"" help:"Migrate a ControllerConfig to a DeploymentRuntimeConfig."`
	PipelineComposition pipelinecomposition.Cmd `cmd:"" help:"Migrate a classic Composition to a Function Pipeline Composition."`
}

// Help returns help message for the migrate command.
func (c *Cmd) Help() string {
	return `
This command migrates a Crossplane resource to a newer version or a different kind.

Currently supported resources:
* ControllerConfig -> DeploymentRuntimeConfig
* Classic Compositions -> Function Pipeline Compositions

Examples:
TODO: Add examples
`
}

// Run runs the trace command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error {
	return nil
}

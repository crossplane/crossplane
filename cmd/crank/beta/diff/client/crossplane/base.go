package crossplane

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
)

func (c *Clients) Initialize(ctx context.Context, logger logging.Logger) error {
	return core.InitializeClients(ctx, logger,
		c.Composition,
		c.Definition,
		c.Environment,
		c.Function,
		c.ResourceTree,
	)
}

type Clients struct {
	Composition  CompositionClient
	Definition   DefinitionClient
	Environment  EnvironmentClient
	Function     FunctionClient
	ResourceTree ResourceTreeClient
}

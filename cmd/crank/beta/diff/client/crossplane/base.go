// Package crossplane contains interfaces and implementations for clients that talk to Kubernetes about Crossplane
// primitives, often by consuming clients from the kubernetes package.
package crossplane

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
)

// Initialize initializes all the clients in this bundle.
func (c *Clients) Initialize(ctx context.Context, logger logging.Logger) error {
	return core.InitializeClients(ctx, logger,
		c.Composition,
		c.Definition,
		c.Environment,
		c.Function,
		c.ResourceTree,
	)
}

// Clients is an aggregation of all of our Crossplane clients, used to pass them as a bundle,
// typically for initialization where the consumer can select which ones they need.
type Clients struct {
	Composition  CompositionClient
	Definition   DefinitionClient
	Environment  EnvironmentClient
	Function     FunctionClient
	ResourceTree ResourceTreeClient
}

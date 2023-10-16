package main

import (
	"fmt"
	"os"

	"golang.org/x/exp/slices"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/internal/graph"
)

const (
	errGetResource = "cannot get requested resource"
	errCliOutput   = "cannot print output"
)

// describeCmd describes a Kubernetes Crossplane resource.
type describeCmd struct {
	Kind      string   `arg:"" required:"" help:"Kind of resource to describe."`
	Name      string   `arg:"" required:"" help:"Name of specified resource to describe."`
	Namespace string   `short:"n" name:"namespace" help:"Namespace of resource to describe." default:"default"`
	Output    string   `short:"o" name:"output" help:"Output type of graph. Possible output types: tree, table, graph." enum:"tree,table,graph" default:"tree"`
	Fields    []string `short:"f" name:"fields" help:"Fields that are printed out in the header." default:"kind,name"`
}

func (c *describeCmd) Run(logger logging.Logger) error {
	logger = logger.WithValues("Kind", c.Kind, "Name", c.Name)

	AllowedFields := []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"}

	// Check if fields are valid
	for _, field := range c.Fields {
		if !slices.Contains(AllowedFields, field) {
			logger.Debug("Invalid field set", "invalidField", field)
			return fmt.Errorf("Invalid field set: %s\nField has to be one of: %s", field, AllowedFields)
		}
	}

	// set kubeconfig
	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	// Get client for k8s package
	client, err := graph.NewClient(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "Couldn't init kubeclient")
	}

	// Init new printer
	p, err := graph.NewPrinter(c.Output)
	if err != nil {
		return errors.Wrap(err, "cannot init new printer")
	}

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	root, err := client.GetResource(c.Kind, c.Name, c.Namespace)
	if err != nil {
		logger.Debug(errGetResource, "error", err)
		return errors.Wrap(err, errGetResource)
	}

	// Print resources
	err = p.Print(os.Stdout, *root, c.Fields)
	if err != nil {
		return errors.Wrap(err, errCliOutput)
	}

	return nil
}

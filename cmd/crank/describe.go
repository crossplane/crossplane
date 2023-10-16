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

//nolint:gocyclo // FFew simple if statements to validate fields and to select output
func (c *describeCmd) Run(logger logging.Logger) error {
	logger = logger.WithValues("Kind", c.Kind, "Name", c.Name)

	allowedFields := []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"}
	allowedOutput := []string{"tree", "table", "graph"}

	// Check if fields are valid
	for _, field := range c.Fields {
		if !slices.Contains(allowedFields, field) {
			logger.Debug("Invalid field set", "invalidField", field)
			return fmt.Errorf("Invalid field set: %s\nField has to be one of: %s", field, allowedFields)
		}
	}

	// Check if output format is valid
	if !slices.Contains(allowedOutput, c.Output) {
		logger.Debug("Invalid output set", "invalidOutput", c.Output)
		return fmt.Errorf("Invalid output set: %s\nOutput has to be one of: %s", c.Output, allowedOutput)
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

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	root, err := graph.GetResource(c.Kind, c.Name, c.Namespace, client)
	if err != nil {
		logger.Debug(errGetResource, "error", err)
		return errors.Wrap(err, errGetResource)
	}

	// Configure printer
	var p graph.Printer

	switch c.Output {
	case "tree":
		p = &graph.Tree{
			Indent: "",
			IsLast: true,
		}
	case "table":
		p = &graph.Table{}
	case "graph":
		p = &graph.DotGraph{}
	}

	// Print resources
	err = p.Print(os.Stdout, *root, c.Fields)
	if err != nil {
		return errors.Wrap(err, errCliOutput)
	}

	return nil
}

package main

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// pushCmd pushes a package.
type describeCmd struct {
	Kind       string   `arg:"" required:"" help:"Kind of resource to describe."`
	Name       string   `arg:"" required:"" help:"Name of specified resource to describe."`
	Namespace  string   `short:"n" name:"namespace" help:"Namespace of resource to describe." default:"default"`
	Kubeconfig string   `short:"k" name:"kubeconfig" help:"Absolute path to kubeconfig."`
	Output     string   `short:"o" name:"output" help:"Output type of graph. Possible output types: cli, graph." enum:"cli,graph" default:"cli"`
	Fields     []string `short:"f" name:"fields" help:"Fields that are printed out in the header." default:"parent,kind,name,synced,ready"`
	OutputPath string   `short:"p" name:"path" help:"Output path for graph PNG. Only valid when set with output=graph."`
}

func (c *describeCmd) Run(logger logging.Logger) error {

	fmt.Printf("Kind: %s, Name: %s, Namespace: %s", c.Kind, c.Name, c.Namespace)
	return nil
}

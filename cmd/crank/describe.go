package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"golang.org/x/exp/slices"
	"k8s.io/client-go/util/homedir"
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
	logger = logger.WithValues("Kind", c.Kind, "Name", c.Name)

	allowedFields := []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"}
	allowedOutput := []string{"cli", "graph"}

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
		return fmt.Errorf("Invalid ouput set: %s\nOutput has to be one of: %s", c.Output, allowedOutput)
	}

	// Get and set kubeconfig
	// 1.Checks flag Kubeconfig 2. Check env var `KUBECONFIG` 3. Check ~/.kube/config dir
	if c.Kubeconfig == "" {
		c.Kubeconfig = os.Getenv("KUBECONFIG")
		logger.Debug("Set Kubeconfig via environment variable `KUBECONFIG`", "kubeconfig-path", c.Kubeconfig)
	}
	if c.Kubeconfig == "" {
		c.Kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
		logger.Debug("Used Kubeconfig file stored in homeDir", "kubeconfig-path", c.Kubeconfig)
	}

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	root, err := resource.GetResource(c.Kind, c.Name, c.Namespace, c.Kubeconfig)
	if err != nil {
		logger.Debug("Couldn't get requested resource.", "error", err)
		return err
	}

	// Print out resource
	switch c.Output {
	case "cli":
		if err := resource.PrintResourceTable(*root, c.Fields); err != nil {
			logger.Debug("Error printing CLI table.", "error", err)
			return err
		}
	case "graph":
		printer := resource.NewGraphPrinter()
		if err := printer.Print(*root, c.Fields, c.OutputPath); err != nil {
			logger.Debug("Error printing CLI table.", "error", err)
			return err
		}
	}

	return nil
}

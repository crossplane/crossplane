package main

import (
	"fmt"
	"reflect"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/crossplane/crossplane/internal/printer"
	"golang.org/x/exp/slices"
	ctrl "sigs.k8s.io/controller-runtime"
)

// diagnoseCmd diagnoses a Kubernetes Crossplane resource.
type diagnoseCmd struct {
	Kind      string   `arg:"" required:"" help:"Kind of resource to diagnose"`
	Name      string   `arg:"" required:"" help:"Name of specified resource to diagnose."`
	Namespace string   `short:"n" name:"namespace" help:"Namespace of resource to diagnose." default:"default"`
	Fields    []string `short:"f" name:"fields" help:"Fields that are printed out in the header." default:"parent,kind,name,synced,ready,message,event"`
}

func (c *diagnoseCmd) Run(logger logging.Logger) error {
	logger = logger.WithValues("Kind", c.Kind, "Name", c.Name)

	allowedFields := []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"}

	// Check if fields are valid
	for _, field := range c.Fields {
		if !slices.Contains(allowedFields, field) {
			logger.Debug("Invalid field set", "invalidField", field)
			return fmt.Errorf("Invalid field set: %s\nField has to be one of: %s", field, allowedFields)
		}
	}

	// set kubeconfig
	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	root, err := k8s.GetResource(c.Kind, c.Name, c.Namespace, kubeconfig)
	if err != nil {
		logger.Debug(errGetResource, "error", err)
		return errors.Wrap(err, errGetResource)
	}

	// Diagnose unhealthy resources
	var unhealthyR k8s.Resource
	unhealthyR, err = k8s.Diagnose(*root, unhealthyR)
	if err != nil {
		return fmt.Errorf("Couldn't finish diagnose -> %w", err)
	}

	if !reflect.DeepEqual(unhealthyR, k8s.Resource{}) {
		// CLI print unhealthy resources
		fmt.Printf("Identified the following resources as potentialy unhealthy.\n")
		if err := printer.CliTable(unhealthyR, c.Fields); err != nil {
			return fmt.Errorf("Error printing CLI table: %w\n", err)
		}
	} else {
		fmt.Printf("Couldn't diagnose any issue with resource %s %s.", root.GetKind(), root.GetName())
	}

	return nil
}

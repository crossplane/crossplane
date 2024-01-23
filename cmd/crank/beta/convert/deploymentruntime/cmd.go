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

// Package deploymentruntime contains the logic for converting a
// ControllerConfig to a DeploymentRuntimeConfig.
package deploymentruntime

import (
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/cmd/crank/beta/convert/io"
)

// Cmd arguments and flags for convert deployment-runtime subcommand.
type Cmd struct {
	// Arguments.
	InputFile string `arg:"" type:"path" optional:"" default:"-" help:"The ControllerConfig file to be Converted. If not specified or '-', stdin will be used."`

	// Flags.
	OutputFile string `short:"o" type:"path" placeholder:"PATH" help:"The file to write the generated DeploymentRuntimeConfig to. If not specified, stdout will be used."`

	fs afero.Fs
}

// Help returns help message for the convert deployment-runtime command.
func (c *Cmd) Help() string {
	return `
This command converts a Crossplane ControllerConfig to a DeploymentRuntimeConfig.

DeploymentRuntimeConfig was introduced in Crossplane 1.14 and ControllerConfig is
deprecated.

Examples:

  # Write out a DeploymentRuntimeConfigFile from a ControllerConfig
  crossplane beta convert deployment-runtime cc.yaml -o drc.yaml

  # Create a new DeploymentRuntimeConfig via Stdout
  crossplane beta convert deployment-runtime cc.yaml | grep -v creationTimestamp | kubectl apply -f - 

`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run converts a ControllerConfig to a DeploymentRuntimeConfig.
func (c *Cmd) Run() error {
	data, err := io.Read(c.fs, c.InputFile)
	if err != nil {
		return err
	}

	// Set up schemes for our API types
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = v1alpha1.AddToScheme(sch)
	_ = v1beta1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	cc := &v1alpha1.ControllerConfig{}
	_, _, err = decode(data, &v1alpha1.ControllerConfigGroupVersionKind, cc)
	if err != nil {
		return errors.Wrap(err, "Decode Error")
	}

	drc, err := controllerConfigToDeploymentRuntimeConfig(cc)
	if err != nil {
		return errors.Wrap(err, "Cannot migrate to Deployment Runtime")
	}

	return io.WriteObjectYAML(c.fs, c.OutputFile, drc)
}

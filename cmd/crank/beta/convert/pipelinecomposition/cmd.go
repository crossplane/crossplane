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

// Package pipelinecomposition is a package for converting
// patch-and-transform Compositions to a function pipeline.
package pipelinecomposition

import (
	"bufio"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane/cmd/crank/beta/convert/compositionenvironment"
	commonio "github.com/crossplane/crossplane/cmd/crank/beta/convert/io"
)

// Cmd arguments and flags for converting a patch-and-transform to a function pipeline composition.
type Cmd struct {
	// Arguments.
	InputFile string `arg:"" default:"-" help:"The Composition file to be converted. If not specified or '-', stdin will be used." optional:"" predictor:"file" type:"path"`

	// Flags.
	OutputFile string `help:"The file to write the generated Composition to. If not specified, stdout will be used." placeholder:"PATH" predictor:"file" short:"o" type:"path"`

	FunctionPatchAndTransformRef string `default:"function-patch-and-transform" help:"Name of the existing function-patch-and-transform Function, to be used to reference it." name:"function-patch-and-transform-ref"`
	FunctionEnvironmentConfigRef string `default:"function-environment-configs" help:"Name of the existing function-environment-configs Function, to be used to reference it." name:"function-environment-configs-ref"`

	fs afero.Fs
}

// Help returns help message for the migrate pipeline-composition command.
func (c *Cmd) Help() string {
	return `
This command converts a Crossplane Composition to use a Composition function pipeline.

By default it transforms the Composition using the classic patch-and-transform approach
to a function pipeline using crossplane-contrib/function-patch-and-transform, but the
function ref name can be overridden with the --function-patch-and-transform-ref flag.

If native Composition Environment was used it will also convert the Composition to use
function-environment-configs, by default it'll reference the function as
function-environment-configs, but it can be overridden with the --function-environment-configs-ref flag.


Examples:

  # Convert an existing Composition to use Pipelines
  crossplane beta convert pipeline-composition composition.yaml -o pipeline-composition.yaml

  # Use a different functionRef for function-patch-and-transform and output to stdout
  crossplane beta convert pipeline-composition composition.yaml --function-patch-and-transform-ref local-function-patch-and-transform

  # Use a different functionRef for function-environment-configs and output to stdout
  crossplane beta convert pipeline-composition composition.yaml --function-environment-configs-ref local-function-environment-configs

  # Stdin to stdout
  cat composition.yaml | ./crossplane beta convert pipeline-composition 

`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run converts a classic Composition to a function pipeline Composition.
func (c *Cmd) Run(k *kong.Context) error {
	data, err := commonio.Read(c.fs, c.InputFile)
	if err != nil {
		return err
	}

	u := &unstructured.Unstructured{}

	if err := yaml.Unmarshal(data, u); err != nil {
		return errors.Wrap(err, "Unmarshalling Error")
	}

	out, err := convertPnTToPipeline(u, c.FunctionPatchAndTransformRef)
	if err != nil {
		return errors.Wrap(err, "Error generating new Composition")
	}
	if out == nil {
		_, err = fmt.Fprintf(k.Stderr, "No changes needed.\n")
		return errors.Wrap(err, "unable to write to stderr")
	}

	outWithEnv, err := compositionenvironment.ConvertToFunctionEnvironmentConfigs(out, c.FunctionEnvironmentConfigRef)
	if err != nil {
		return errors.Wrap(err, "Error generating new Composition")
	}
	if outWithEnv != nil {
		out = outWithEnv
	}

	b, err := yaml.Marshal(out)
	if err != nil {
		return errors.Wrap(err, "Unable to marshal back to yaml")
	}

	output := k.Stdout
	if outputFileName := c.OutputFile; outputFileName != "" {
		f, err := c.fs.OpenFile(outputFileName, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return errors.Wrap(err, "Unable to open output file")
		}
		defer func() { _ = f.Close() }()
		output = f
	}

	outputW := bufio.NewWriter(output)
	if _, err := outputW.WriteString("---\n"); err != nil {
		return errors.Wrap(err, "Writing YAML file header")
	}

	if _, err := outputW.Write(b); err != nil {
		return errors.Wrap(err, "Writing YAML file content")
	}

	if err := outputW.Flush(); err != nil {
		return errors.Wrap(err, "Flushing output")
	}

	return nil
}

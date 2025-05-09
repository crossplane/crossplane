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

// Package compositionenvironment is a package for converting Pipeline Compositions using native Composition Environment
// capabilities to use function-environment-configs.
package compositionenvironment

import (
	"bufio"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	commonIO "github.com/crossplane/crossplane/cmd/crank/beta/convert/io"
)

// Cmd arguments and flags for converting a Composition to use function-environment-configs.
type Cmd struct {
	// Arguments.
	InputFile string `arg:"" default:"-" help:"The Composition file to be converted. If not specified or '-', stdin will be used." optional:"" predictor:"file" type:"path"`

	// Flags.
	OutputFile string `help:"The file to write the generated Composition to. If not specified, stdout will be used." placeholder:"PATH" predictor:"file" short:"o" type:"path"`

	FunctionEnvironmentConfigRef string `default:"function-environment-configs" help:"Name of the existing function-environment-configs Function, to be used to reference it." name:"function-environment-configs-ref"`

	fs afero.Fs
}

// Help returns help message for the migrate composition-environment command.
func (c *Cmd) Help() string {
	return `
This command converts a Crossplane Composition to use function-environment-configs, if needed.

It adds a function pipeline step using crossplane-contrib/function-environment-configs, if needed.
By default it'll reference the function as function-environment-configs, but it can be overridden
with the -f flag.

Examples:

  # Convert an existing Composition (Pipeline mode) leveraging native
  # Composition Environment to use function-environment-configs.
  crossplane beta convert composition-environment composition.yaml -o composition-environment.yaml

  # Use a different functionRef and output to stdout.
  crossplane beta convert composition-environment composition.yaml --function-environment-configs-ref local-function-environment-configs

  # Stdin to stdout.
  cat composition.yaml | ./crossplane beta convert composition-environment

`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run converts a classic Composition to a function pipeline Composition.
func (c *Cmd) Run(k *kong.Context) error {
	data, err := commonIO.Read(c.fs, c.InputFile)
	if err != nil {
		return err
	}

	u := &unstructured.Unstructured{}

	if err := yaml.Unmarshal(data, u); err != nil {
		return errors.Wrap(err, "Unmarshalling Error")
	}

	out, err := ConvertToFunctionEnvironmentConfigs(u, c.FunctionEnvironmentConfigRef)
	if err != nil {
		return errors.Wrap(err, "Error generating new Composition")
	}
	if out == nil {
		_, err = fmt.Fprintf(k.Stderr, "No changes needed.\n")
		return errors.Wrap(err, "unable to write to stderr")
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

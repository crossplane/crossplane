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
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/convert/io"
)

// Cmd arguments and flags for converting a patch-and-transform to a function pipeline composition.
type Cmd struct {
	// Arguments.
	InputFile string `arg:"" type:"path" optional:"" default:"-" help:"The Composition file to be converted. If not specified or '-', stdin will be used."`

	// Flags.
	OutputFile   string `short:"o" type:"path" placeholder:"PATH" help:"The file to write the generated Composition to. If not specified, stdout will be used."`
	FunctionName string `short:"f" type:"string" placeholder:"STRING" help:"FunctionRefName. Defaults to function-patch-and-transform."`

	fs afero.Fs
}

// Help returns help message for the migrate pipeline-composition command.
func (c *Cmd) Help() string {
	return `
This command converts a Crossplane Composition to use a Composition function pipeline.

By default it transforms the Composition using the classic patch-and-transform approach
to a function pipeline using crossplane-contrib/function-patch-and-transform, but the
function ref name can be overridden with the -f flag.

Examples:

  # Convert an existing Composition to use Pipelines
  crossplane beta convert pipeline-composition composition.yaml -o pipeline-composition.yaml

  # Use a different functionRef and output to stdout
  crossplane beta convert pipeline-composition composition.yaml -f local-function-patch-and-transform

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
func (c *Cmd) Run() error {
	data, err := io.Read(c.fs, c.InputFile)
	if err != nil {
		return err
	}

	// Set up schemes for our API types
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = v1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	oc := &v1.Composition{}
	_, _, err = decode(data, &v1.CompositionGroupVersionKind, oc)
	if err != nil {
		return errors.Wrap(err, "Decoding Error")
	}

	_, errs := oc.Validate()
	if len(errs) > 0 {
		return errors.Wrap(errs.ToAggregate(), "Existing Composition Validation error")
	}

	pc, err := convertPnTToPipeline(oc, c.FunctionName)
	if err != nil {
		return errors.Wrap(err, "Error generating new Composition")
	}

	return io.WriteObjectYAML(c.fs, c.OutputFile, pc)
}

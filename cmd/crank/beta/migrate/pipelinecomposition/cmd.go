/*
Copyright 2023 The Crossplane Authors.

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

package pipelinecomposition

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// Cmd arguments and flags for migrating a classic to a function pipeline composition.
type Cmd struct {
	// Arguments.
	InputFile string `short:"i" type:"path" placeholder:"PATH" help:"The Composition file to be converted."`

	OutputFile   string `short:"o" type:"path" placeholder:"PATH" help:"The file to write the generated Composition to."`
	FunctionName string `short:"f" type:"string" placeholder:"STRING" help:"FunctionRefName. Defaults to function-patch-and-transform."`
}

func (c *Cmd) Help() string {
	return `
This command converts a Crossplane Composition to use a Composition function pipeline.


Examples:

  # Convert an existing Composition to use Pipelines

  crossplane-migrator new-pipeline-composition -i composition.yaml -o new-composition.yaml

  # Use a different functionRef and output to stdout

  crossplane-migrator new-pipeline-composition -i composition.yaml -f local-function-patch-and-transform

  # Stdin to stdout

  cat composition.yaml | ./crossplane-migrator new-pipeline-composition 

`
}

func (c *Cmd) Run() error {
	var data []byte
	var err error

	if c.InputFile != "" {
		data, err = os.ReadFile(c.InputFile)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return errors.Wrap(err, "Unable to read input")
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

	pc, err := NewPipelineCompositionFromExisting(oc, c.FunctionName)
	if err != nil {
		return errors.Wrap(err, "Error generating new Composition")
	}

	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})

	var output io.Writer

	if c.OutputFile != "" {
		f, err := os.OpenFile(c.OutputFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return errors.Wrap(err, "Unable to open output file")
		}
		defer f.Close()
		output = f
	} else {
		output = os.Stdout
	}

	err = s.Encode(pc, output)
	if err != nil {
		return errors.Wrap(err, "Unable to encode output")
	}
	return nil
}

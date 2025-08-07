/*
Copyright 2025 The Crossplane Authors.

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

// Package op implements operation rendering using operation functions.
package op

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/cmd/crank/render"
)

// Cmd arguments and flags for alpha render op subcommand.
type Cmd struct {
	// Arguments.
	Operation string `arg:"" help:"A YAML file specifying the Operation to render."                                                           predictor:"yaml_file"              type:"existingfile"`
	Functions string `arg:"" help:"A YAML file or directory of YAML files specifying the operation functions to use to render the Operation." predictor:"yaml_file_or_directory" type:"path"`

	// Flags. Keep them in alphabetical order.
	ContextFiles           map[string]string `help:"Comma-separated context key-value pairs to pass to the function pipeline. Values must be files containing JSON."                           mapsep:""          predictor:"file"`
	ContextValues          map[string]string `help:"Comma-separated context key-value pairs to pass to the function pipeline. Values must be JSON. Keys take precedence over --context-files." mapsep:""`
	FunctionCredentials    string            `help:"A YAML file or directory of YAML files specifying credentials to use for functions."                                                       placeholder:"PATH" predictor:"yaml_file_or_directory" type:"path"`
	IncludeContext         bool              `help:"Include the context in the rendered output as a resource of kind: Context."                                                                short:"c"`
	IncludeFullOperation   bool              `help:"Include a direct copy of the input Operation's spec and metadata fields in the rendered output."                                           short:"o"`
	IncludeFunctionResults bool              `help:"Include informational and warning messages from functions in the rendered output as resources of kind: Result."                            short:"r"`
	RequiredResources      string            `help:"A YAML file or directory of YAML files specifying required resources to pass to the function pipeline."                                    placeholder:"PATH" predictor:"yaml_file_or_directory" short:"e"   type:"path"`

	Timeout time.Duration `default:"1m" help:"How long to run before timing out."`

	fs afero.Fs
}

// Help prints out the help for the alpha render op command.
func (c *Cmd) Help() string {
	return `
This command shows you what resources an Operation would create or mutate by
printing them to stdout. It doesn't talk to Crossplane. Instead it runs the
operation function pipeline specified by the Operation locally.

For Operations, it runs the operation function pipeline directly and shows what
resources the operation would mutate.

Functions are pulled and run using Docker by default. You can add
the following annotations to each function to change how they're run:

  render.crossplane.io/runtime: "Development"

    Connect to a function that is already running, instead of using Docker. This
	is useful to develop and debug new functions. The function must be listening
	at localhost:9443 and running with the --insecure flag.

  render.crossplane.io/runtime-development-target: "dns:///example.org:7443"

    Connect to a function running somewhere other than localhost:9443. The
	target uses gRPC target syntax.

  render.crossplane.io/runtime-docker-cleanup: "Orphan"

    Don't stop the function's Docker container after rendering.

  render.crossplane.io/runtime-docker-name: "<name>"

    create a container with that name and also reuse it as long as it is running or can be restarted.

  render.crossplane.io/runtime-docker-pull-policy: "Always"

    Always pull the function's package, even if it already exists locally.
	Other supported values are Never, or IfNotPresent.

Use the standard DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and
DOCKER_TLS_VERIFY environment variables to configure how this command connects
to the Docker daemon.

Examples:

  # Render an Operation.
  crossplane alpha render op operation.yaml functions.yaml

  # Pass context values to the function pipeline.
  crossplane alpha render op operation.yaml functions.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'

  # Pass required resources functions can request.
  crossplane alpha render op operation.yaml functions.yaml \
	--required-resources=required-resources.yaml

  # Pass credentials to functions that need them.
  crossplane alpha render op operation.yaml functions.yaml \
	--function-credentials=credentials.yaml

  # Include function results and context in output.
  crossplane alpha render op operation.yaml functions.yaml -r -c

  # Include the full Operation with original spec and metadata.
  crossplane alpha render op operation.yaml functions.yaml -o
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run alpha render op.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// Load operation
	op, err := LoadOperation(c.fs, c.Operation)
	if err != nil {
		return err
	}

	// Load functions
	fns, err := render.LoadFunctions(c.fs, c.Functions)
	if err != nil {
		return err
	}

	// Load function credentials
	fcreds := []corev1.Secret{}
	if c.FunctionCredentials != "" {
		fcreds, err = render.LoadCredentials(c.fs, c.FunctionCredentials)
		if err != nil {
			return errors.Wrapf(err, "cannot load function credentials from %q", c.FunctionCredentials)
		}
	}

	// Load required resources
	rrs := []unstructured.Unstructured{}
	if c.RequiredResources != "" {
		rrs, err = render.LoadRequiredResources(c.fs, c.RequiredResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load required resources from %q", c.RequiredResources)
		}
	}

	// Build context data from files and values
	contextData := make(map[string][]byte)
	for k, filename := range c.ContextFiles {
		data, err := afero.ReadFile(c.fs, filename)
		if err != nil {
			return errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		contextData[k] = data
	}

	for k, data := range c.ContextValues {
		contextData[k] = []byte(data)
	}

	// Render the operation
	out, err := Render(ctx, log, Inputs{
		Operation:           op,
		Functions:           fns,
		FunctionCredentials: fcreds,
		RequiredResources:   rrs,
		Context:             contextData,
	})
	if err != nil {
		return err
	}

	// Output results
	s := kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, nil, nil, kjson.SerializerOptions{Yaml: true})

	// Only include spec when IncludeFullOperation flag is set
	if c.IncludeFullOperation {
		_ = fieldpath.Pave(out.Operation.Object).SetValue("spec", *op.Spec.DeepCopy())
	}

	// Always output the Operation (with metadata and status, optionally with spec)
	_, _ = fmt.Fprintln(k.Stdout, "---")
	if err := s.Encode(out.Operation, k.Stdout); err != nil {
		return errors.Wrapf(err, "cannot marshal operation %q to YAML", op.GetName())
	}

	// Output rendered resources
	for i := range out.Resources {
		_, _ = fmt.Fprintln(k.Stdout, "---")
		if err := s.Encode(&out.Resources[i], k.Stdout); err != nil {
			return errors.Wrap(err, "cannot marshal desired resource to YAML")
		}
	}

	// Output results if requested
	if c.IncludeFunctionResults {
		for i := range out.Results {
			_, _ = fmt.Fprintln(k.Stdout, "---")
			if err := s.Encode(&out.Results[i], k.Stdout); err != nil {
				return errors.Wrap(err, "cannot marshal result to YAML")
			}
		}
	}

	// Output context if requested
	if c.IncludeContext && out.Context != nil {
		_, _ = fmt.Fprintln(k.Stdout, "---")
		if err := s.Encode(out.Context, k.Stdout); err != nil {
			return errors.Wrap(err, "cannot marshal context to YAML")
		}
	}

	return nil
}

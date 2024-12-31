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

// Package render implements composition rendering using composition functions.
package render

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments.
	CompositeResource string `arg:"" help:"A YAML file specifying the composite resource (XR) to render."                                        type:"existingfile"`
	Composition       string `arg:"" help:"A YAML file specifying the Composition to use to render the XR. Must be mode: Pipeline."              type:"existingfile"`
	Functions         string `arg:"" help:"A YAML file or directory of YAML files specifying the Composition Functions to use to render the XR." type:"path"`

	// Flags. Keep them in alphabetical order.
	ContextFiles           map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be files containing JSON."                           mapsep:""`
	ContextValues          map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be JSON. Keys take precedence over --context-files." mapsep:""`
	IncludeFunctionResults bool              `help:"Include informational and warning messages from Functions in the rendered output as resources of kind: Result."                            short:"r"`
	IncludeFullXR          bool              `help:"Include a direct copy of the input XR's spec and metadata fields in the rendered output."                                                  short:"x"`
	ObservedResources      string            `help:"A YAML file or directory of YAML files specifying the observed state of composed resources."                                               placeholder:"PATH" short:"o"   type:"path"`
	ExtraResources         string            `help:"A YAML file or directory of YAML files specifying extra resources to pass to the Function pipeline."                                       placeholder:"PATH" short:"e"   type:"path"`
	IncludeContext         bool              `help:"Include the context in the rendered output as a resource of kind: Context."                                                                short:"c"`
	FunctionCredentials    string            `help:"A YAML file or directory of YAML files specifying credentials to use for Functions to render the XR."                                      placeholder:"PATH" type:"path"`

	Timeout time.Duration `default:"1m" help:"How long to run before timing out."`

	fs afero.Fs
}

// Help prints out the help for the render command.
func (c *Cmd) Help() string {
	return `
This command shows you what composed resources Crossplane would create by
printing them to stdout. It also prints any changes that would be made to the
status of the XR. It doesn't talk to Crossplane. Instead it runs the Composition
Function pipeline specified by the Composition locally, and uses that to render
the XR. It only supports Compositions in Pipeline mode.

Composition Functions are pulled and run using Docker by default. You can add
the following annotations to each Function to change how they're run:

  render.crossplane.io/runtime: "Development"

    Connect to a Function that is already running, instead of using Docker. This
	is useful to develop and debug new Functions. The Function must be listening
	at localhost:9443 and running with the --insecure flag.

  render.crossplane.io/runtime-development-target: "dns:///example.org:7443"

    Connect to a Function running somewhere other than localhost:9443. The
	target uses gRPC target syntax.

  render.crossplane.io/runtime-docker-cleanup: "Orphan"

    Don't stop the Function's Docker container after rendering.

  render.crossplane.io/runtime-docker-name: "<name>"

    create a container with that name and also reuse it as long as it is running or can be restarted.

  render.crossplane.io/runtime-docker-pull-policy: "Always"

    Always pull the Function's package, even if it already exists locally.
	Other supported values are Never, or IfNotPresent.

Use the standard DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and
DOCKER_TLS_VERIFY environment variables to configure how this command connects
to the Docker daemon.

Examples:

  # Simulate creating a new XR.
  crossplane render xr.yaml composition.yaml functions.yaml

  # Simulate updating an XR that already exists.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --observed-resources=existing-observed-resources.yaml

  # Pass context values to the Function pipeline.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'

  # Pass extra resources Functions in the pipeline can request.
  crossplane render xr.yaml composition.yaml functions.yaml \
	--extra-resources=extra-resources.yaml

  # Pass credentials to Functions in the pipeline that need them.
  crossplane render xr.yaml composition.yaml functions.yaml \
	--function-credentials=credentials.yaml
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run render.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error { //nolint:gocognit // Only a touch over.
	xr, err := LoadCompositeResource(c.fs, c.CompositeResource)
	if err != nil {
		return errors.Wrapf(err, "cannot load composite resource from %q", c.CompositeResource)
	}

	comp, err := LoadComposition(c.fs, c.Composition)
	if err != nil {
		return errors.Wrapf(err, "cannot load Composition from %q", c.Composition)
	}

	// Validate that Composition's compositeTypeRef matches the XR's GroupVersionKind.
	xrGVK := xr.GetObjectKind().GroupVersionKind()
	compRef := comp.Spec.CompositeTypeRef

	if compRef.Kind != xrGVK.Kind {
		return errors.Errorf("composition's compositeTypeRef.kind (%s) does not match XR's kind (%s)", compRef.Kind, xrGVK.Kind)
	}

	if compRef.APIVersion != xrGVK.GroupVersion().String() {
		return errors.Errorf("composition's compositeTypeRef.apiVersion (%s) does not match XR's apiVersion (%s)", compRef.APIVersion, xrGVK.GroupVersion().String())
	}

	warns, errs := comp.Validate()
	for _, warn := range warns {
		_, _ = fmt.Fprintf(k.Stderr, "WARN(composition): %s\n", warn)
	}
	if len(errs) > 0 {
		return errors.Wrapf(errs.ToAggregate(), "invalid Composition %q", comp.GetName())
	}

	// check if XR's matchLabels have corresponding label at composition
	xrSelector := xr.GetCompositionSelector()
	if xrSelector != nil {
		for key, value := range xrSelector.MatchLabels {
			compValue, exists := comp.Labels[key]
			if !exists {
				return fmt.Errorf("composition %q is missing required label %q", comp.GetName(), key)
			}
			if compValue != value {
				return fmt.Errorf("composition %q has incorrect value for label %q: want %q, got %q",
					comp.GetName(), key, value, compValue)
			}
		}
	}

	if m := comp.Spec.Mode; m == nil || *m != v1.CompositionModePipeline {
		return errors.Errorf("render only supports Composition Function pipelines: Composition %q must use spec.mode: Pipeline", comp.GetName())
	}

	fns, err := LoadFunctions(c.fs, c.Functions)
	if err != nil {
		return errors.Wrapf(err, "cannot load functions from %q", c.Functions)
	}

	fcreds := []corev1.Secret{}
	if c.FunctionCredentials != "" {
		fcreds, err = LoadCredentials(c.fs, c.FunctionCredentials)
		if err != nil {
			return errors.Wrapf(err, "cannot load secrets from %q", c.FunctionCredentials)
		}
	}

	ors := []composed.Unstructured{}
	if c.ObservedResources != "" {
		ors, err = LoadObservedResources(c.fs, c.ObservedResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load observed composed resources from %q", c.ObservedResources)
		}
	}

	ers := []unstructured.Unstructured{}
	if c.ExtraResources != "" {
		ers, err = LoadExtraResources(c.fs, c.ExtraResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load extra resources from %q", c.ExtraResources)
		}
	}

	fctx := map[string][]byte{}
	for k, filename := range c.ContextFiles {
		v, err := afero.ReadFile(c.fs, filename)
		if err != nil {
			return errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		fctx[k] = v
	}
	for k, v := range c.ContextValues {
		fctx[k] = []byte(v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	out, err := Render(ctx, log, Inputs{
		CompositeResource:   xr,
		Composition:         comp,
		Functions:           fns,
		FunctionCredentials: fcreds,
		ObservedResources:   ors,
		ExtraResources:      ers,
		Context:             fctx,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render composite resource")
	}

	// TODO(negz): Right now we're just emitting the desired state, which is an
	// overlay on the observed state. Would it be more useful to apply the
	// overlay to show something more like what the final result would be? The
	// challenge with that would be that we'd have to try emulate what
	// server-side apply would do (e.g. merging vs atomically replacing arrays)
	// and we don't have enough context (i.e. OpenAPI schemas) to do that.

	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: true})

	if c.IncludeFullXR {
		xrSpec, err := fieldpath.Pave(xr.Object).GetValue("spec")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource spec")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("spec", xrSpec); err != nil {
			return errors.Wrapf(err, "cannot set composite resource spec")
		}

		xrMeta, err := fieldpath.Pave(xr.Object).GetValue("metadata")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource metadata")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("metadata", xrMeta); err != nil {
			return errors.Wrapf(err, "cannot set composite resource metadata")
		}
	}

	_, _ = fmt.Fprintln(k.Stdout, "---")
	if err := s.Encode(out.CompositeResource, k.Stdout); err != nil {
		return errors.Wrapf(err, "cannot marshal composite resource %q to YAML", xr.GetName())
	}

	for i := range out.ComposedResources {
		_, _ = fmt.Fprintln(k.Stdout, "---")
		if err := s.Encode(&out.ComposedResources[i], k.Stdout); err != nil {
			return errors.Wrapf(err, "cannot marshal composed resource %q to YAML", out.ComposedResources[i].GetAnnotations()[AnnotationKeyCompositionResourceName])
		}
	}

	if c.IncludeFunctionResults {
		for i := range out.Results {
			_, _ = fmt.Fprintln(k.Stdout, "---")
			if err := s.Encode(&out.Results[i], k.Stdout); err != nil {
				return errors.Wrap(err, "cannot marshal result to YAML")
			}
		}
	}

	if c.IncludeContext {
		_, _ = fmt.Fprintln(k.Stdout, "---")
		if err := s.Encode(out.Context, k.Stdout); err != nil {
			return errors.Wrap(err, "cannot marshal context to YAML")
		}
	}

	return nil
}

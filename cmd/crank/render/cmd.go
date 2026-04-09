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
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xcrd"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	EngineFlags `prefix:""`

	// Arguments.
	CompositeResource string `arg:"" help:"A YAML file specifying the composite resource (XR) to render."                                        predictor:"yaml_file"              type:"existingfile"`
	Composition       string `arg:"" help:"A YAML file specifying the Composition to use to render the XR. Must be mode: Pipeline."              predictor:"yaml_file"              type:"existingfile"`
	Functions         string `arg:"" help:"A YAML file or directory of YAML files specifying the Composition Functions to use to render the XR." predictor:"yaml_file_or_directory" type:"path"`

	// Flags. Keep them in alphabetical order.
	ContextFiles           map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be files containing JSON/YAML."                           mapsep:""               predictor:"file"`
	ContextValues          map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be JSON/YAML. Keys take precedence over --context-files." mapsep:""`
	IncludeFunctionResults bool              `help:"Include informational and warning messages from Functions in the rendered output as resources of kind: Result."                                 short:"r"`
	IncludeFullXR          bool              `help:"Include a direct copy of the input XR's spec and metadata fields in the rendered output."                                                       short:"x"`
	ObservedResources      string            `help:"A YAML file or directory of YAML files specifying the observed state of composed resources."                                                    placeholder:"PATH"      predictor:"yaml_file_or_directory" short:"o"   type:"path"`
	ExtraResources         string            `help:"A YAML file or directory of YAML files specifying required resources (deprecated, use --required-resources)."                                   placeholder:"PATH"      predictor:"yaml_file_or_directory" type:"path"`
	RequiredResources      string            `help:"A YAML file or directory of YAML files specifying required resources to pass to the Function pipeline."                                         placeholder:"PATH"      predictor:"yaml_file_or_directory" short:"e"   type:"path"`
	RequiredSchemas        string            `help:"A directory of JSON files specifying OpenAPI v3 schemas (from kubectl get --raw /openapi/v3/<group-version>)."                                  placeholder:"DIR"       predictor:"directory"              short:"s"   type:"path"`
	IncludeContext         bool              `help:"Include the context in the rendered output as a resource of kind: Context."                                                                     short:"c"`
	FunctionCredentials    string            `help:"A YAML file or directory of YAML files specifying credentials to use for Functions to render the XR."                                           placeholder:"PATH"      predictor:"yaml_file_or_directory" type:"path"`
	FunctionAnnotations    []string          `help:"Override function annotations for all functions. Can be repeated."                                                                              placeholder:"KEY=VALUE" short:"a"`

	Timeout time.Duration `default:"1m"                                                                                                     help:"How long to run before timing out."`
	XRD     string        `help:"A YAML file specifying the CompositeResourceDefinition (XRD) that defines the XR's schema and properties." optional:""                               placeholder:"PATH" type:"existingfile"`

	fs afero.Fs
}

// Help prints out the help for the render command.
func (c *Cmd) Help() string {
	return `
This command shows you what composed resources Crossplane would create by
printing them to stdout. It also prints any changes that would be made to the
status of the XR. It runs the Crossplane render engine (either in a Docker
container or via a local binary) to produce high-fidelity output that matches
what the real reconciler would produce.

Composition Functions are pulled and run using Docker by default. You can add
the following annotations to each Function to change how they're run:

  render.crossplane.io/runtime: "Development"

    Connect to a Function that is already running, instead of using Docker. This
	is useful to develop and debug new Functions. The Function must be listening
	at localhost:9443 and running with the --insecure flag.

  render.crossplane.io/runtime-development-target: "dns:///example.org:7443"

    Connect to a Function running somewhere other than localhost:9443. The
	target uses gRPC target syntax (e.g., dns:///example.org:7443 or simply example.org:7443).

  render.crossplane.io/runtime-docker-cleanup: "Orphan"

    Don't stop the Function's Docker container after rendering.

  render.crossplane.io/runtime-docker-name: "<name>"

    create a container with that name and also reuse it as long as it is running or can be restarted.

  render.crossplane.io/runtime-docker-pull-policy: "Always"

    Always pull the Function's package, even if it already exists locally.
	Other supported values are Never, or IfNotPresent.

  render.crossplane.io/runtime-docker-publish-address: "0.0.0.0"

    Host address that Docker should publish the Function's container port to.
    Defaults to 127.0.0.1 (localhost only). Use 0.0.0.0 to publish to all host
    network interfaces, enabling access from remote machines.

  render.crossplane.io/runtime-docker-target: "docker-host"

    Address that the render CLI should use to connect to the Function's Docker
    container. If not specified, uses the publish address.

Use the standard DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and
DOCKER_TLS_VERIFY environment variables to configure how this command connects
to the Docker daemon.

Examples:

  # Simulate creating a new XR.
  crossplane render xr.yaml composition.yaml functions.yaml

  # Simulate updating an XR that already exists.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --observed-resources=existing-observed-resources.yaml

  # Pin the Crossplane version used for rendering.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --crossplane-version=v2.3.0

  # Use a local crossplane binary instead of Docker.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --crossplane-binary=/usr/local/bin/crossplane

  # Pass context values to the Function pipeline.
  crossplane render xr.yaml composition.yaml functions.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'

  # Pass required resources Functions in the pipeline can request.
  crossplane render xr.yaml composition.yaml functions.yaml \
	--required-resources=required-resources.yaml

  # Pass OpenAPI schemas for Functions that need them.
  crossplane render xr.yaml composition.yaml functions.yaml \
	--required-schemas=schemas/

  # Pass credentials to Functions in the pipeline that need them.
  crossplane render xr.yaml composition.yaml functions.yaml \
	--function-credentials=credentials.yaml

  # Override function annotations for a remote Docker daemon.
  DOCKER_HOST=tcp://192.168.1.100:2376 crossplane render xr.yaml composition.yaml functions.yaml \
	-a render.crossplane.io/runtime-docker-publish-address=0.0.0.0 \
	-a render.crossplane.io/runtime-docker-target=192.168.1.100

  # Force all functions to use development runtime.
  crossplane render xr.yaml composition.yaml functions.yaml \
	-a render.crossplane.io/runtime=Development \
	-a render.crossplane.io/runtime-development-target=localhost:9444
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run render.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error { //nolint:gocognit // Orchestration is inherently complex.
	// Warn about flags not yet supported by the render engine.
	// TODO(negz): Extend the render proto to support these.
	if c.IncludeContext {
		log.Info("Warning: --include-context is not yet supported by the render engine and will be ignored")
	}

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

	if comp.Spec.Mode != v1.CompositionModePipeline {
		return errors.Errorf("render only supports Composition Function pipelines: Composition %q must use spec.mode: Pipeline", comp.GetName())
	}

	fns, err := LoadFunctions(c.fs, c.Functions)
	if err != nil {
		return errors.Wrapf(err, "cannot load functions from %q", c.Functions)
	}

	// Apply global annotation overrides to each function
	if err := OverrideFunctionAnnotations(fns, c.FunctionAnnotations); err != nil {
		return errors.Wrap(err, "cannot apply function annotation overrides")
	}

	if c.XRD != "" {
		xrd, err := LoadXRD(c.fs, c.XRD)
		if err != nil {
			return errors.Wrapf(err, "cannot load XRD from %q", c.XRD)
		}

		crd, err := xcrd.ForCompositeResource(xrd)
		if err != nil {
			return errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
		}

		if err := DefaultValues(xr.UnstructuredContent(), xr.GetAPIVersion(), *crd); err != nil {
			return errors.Wrapf(err, "cannot default values for XR %q", xr.GetName())
		}
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
		ers, err = LoadRequiredResources(c.fs, c.ExtraResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load extra resources from %q", c.ExtraResources)
		}
	}

	rrs := []unstructured.Unstructured{}
	if c.RequiredResources != "" {
		rrs, err = LoadRequiredResources(c.fs, c.RequiredResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load required resources from %q", c.RequiredResources)
		}
	}

	// Merge extra resources into required resources.
	rrs = append(rrs, ers...)

	// Load required schemas
	rsc := []spec3.OpenAPI{}
	if c.RequiredSchemas != "" {
		rsc, err = LoadRequiredSchemas(c.fs, c.RequiredSchemas)
		if err != nil {
			return errors.Wrapf(err, "cannot load required schemas from %q", c.RequiredSchemas)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	engine := c.NewEngine(log)
	cleanup, err := engine.Setup(ctx, fns)
	if err != nil {
		return err
	}
	defer cleanup()

	// Start function runtimes to get their addresses.
	fnAddrs, err := StartFunctionRuntimes(ctx, log, fns)
	if err != nil {
		return errors.Wrap(err, "cannot start function runtimes")
	}
	defer StopFunctionRuntimes(log, fnAddrs)

	// Build and execute the render request.
	in := CompositionInputs{
		CompositeResource:   xr,
		Composition:         comp,
		FunctionAddrs:       fnAddrs.Addresses(),
		ObservedResources:   ors,
		RequiredResources:   rrs,
		RequiredSchemas:     rsc,
		FunctionCredentials: fcreds,
	}
	req, err := buildCompositeRequest(in)
	if err != nil {
		return errors.Wrap(err, "cannot build render request")
	}

	rsp, err := engine.Render(ctx, req)
	if err != nil {
		return errors.Wrap(err, "cannot render composite resource")
	}

	compositeOut := rsp.GetComposite()
	if compositeOut == nil {
		return errors.New("render response does not contain a composite output")
	}

	out, err := parseCompositeResponse(compositeOut)
	if err != nil {
		return errors.Wrap(err, "cannot parse render response")
	}

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

	return nil
}

// OverrideFunctionAnnotations applies annotation overrides from flags to
// functions.
func OverrideFunctionAnnotations(fns []pkgv1.Function, annotations []string) error {
	for i := range fns {
		if fns[i].Annotations == nil {
			fns[i].Annotations = make(map[string]string)
		}
		for _, annotation := range annotations {
			parts := strings.SplitN(annotation, "=", 2)
			if len(parts) != 2 {
				return errors.Errorf("invalid function annotation format %q, expected key=value", annotation)
			}
			key, value := parts[0], parts[1]
			fns[i].Annotations[key] = value // Flags override existing annotations
		}
	}
	return nil
}

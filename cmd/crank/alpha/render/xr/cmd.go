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

// Package xr implements XR rendering by delegating to the existing render command.
package xr

import (
	"github.com/alecthomas/kong"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/cmd/crank/render"
)

// Cmd renders a composite resource (XR) by delegating to the existing render command.
type Cmd struct {
	render.Cmd
}

// Help prints out the help for the alpha render xr command.
func (c *Cmd) Help() string {
	return `
This command renders a composite resource (XR) by delegating to the main
crossplane render command. It supports all the same functionality.

For composite resources (XRs), it requires a Composition in Pipeline mode and
renders the XR using composition functions.

Functions are pulled and run using Docker by default. You can add
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

  # Render a composite resource.
  crossplane alpha render xr xr.yaml composition.yaml functions.yaml

  # Simulate updating an XR that already exists.
  crossplane alpha render xr xr.yaml composition.yaml functions.yaml \
    --observed-resources=existing-observed-resources.yaml

  # Pass context values to the Function pipeline.
  crossplane alpha render xr xr.yaml composition.yaml functions.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'

  # Pass required resources Functions can request.
  crossplane alpha render xr xr.yaml composition.yaml functions.yaml \
	--required-resources=required-resources.yaml

  # Pass credentials to Functions that need them.
  crossplane alpha render xr xr.yaml composition.yaml functions.yaml \
	--function-credentials=credentials.yaml
`
}

// Run delegates to the existing render command.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error {
	return c.Cmd.Run(k, log)
}

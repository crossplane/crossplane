/*
Copyright 2026 The Crossplane Authors.

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

package render

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/docker"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// dockerRenderEngine executes crossplane internal render in a Docker container.
type dockerRenderEngine struct {
	// image is the Crossplane Docker image reference.
	image string
	// network is the Docker network to connect the container to. When set,
	// the container joins this network so it can reach function containers.
	network string

	log logging.Logger
}

// Setup creates a temporary Docker network, records its name so the render
// container joins it, and annotates the supplied functions so their
// containers also join it. The returned cleanup function removes the
// network.
func (e *dockerRenderEngine) Setup(ctx context.Context, fns []pkgv1.Function) (func(), error) {
	networkID, networkName, err := createRenderNetwork(ctx)
	if err != nil {
		return func() {}, errors.Wrap(err, "cannot create Docker network for rendering")
	}

	e.network = networkName
	injectNetworkAnnotation(fns, networkName)

	cleanup := func() { //nolint:contextcheck // Detached context for cleanup.
		_ = removeRenderNetwork(context.Background(), networkID)
	}

	return cleanup, nil
}

// Render marshals the request, runs it through a Docker container, and returns
// the response.
func (e *dockerRenderEngine) Render(ctx context.Context, req *renderv1alpha1.RenderRequest) (*renderv1alpha1.RenderResponse, error) {
	// Update any localhost function addresses if needed.
	if cinput := req.GetComposite(); cinput != nil {
		cinput.Functions = RewriteAddressesForDocker(cinput.GetFunctions())
	}
	if oinput := req.GetOperation(); oinput != nil {
		oinput.Functions = RewriteAddressesForDocker(oinput.GetFunctions())
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal render request")
	}

	opts := []docker.RunContainerOption{
		docker.RunWithCommand([]string{"internal", "render"}),
		docker.RunWithStdin(data),
		// Let the container access any functions running in development mode on
		// the host.
		docker.RunWithExtraHosts([]string{"host.docker.internal:host-gateway"}),
	}
	if e.network != "" {
		opts = append(opts, docker.RunWithNetworkName(e.network))
	}

	e.log.Debug("Running crossplane internal render in Docker", "image", e.image, "network", e.network)

	stdout, _, err := docker.RunContainer(ctx, e.image, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "cannot run crossplane internal render in Docker")
	}

	rsp := &renderv1alpha1.RenderResponse{}
	if err := proto.Unmarshal(stdout, rsp); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal render response")
	}

	return rsp, nil
}

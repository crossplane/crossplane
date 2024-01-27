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

package render

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// Annotations that can be used to configure the Docker runtime.
const (
	// AnnotationKeyRuntimeDockerCleanup configures how a Function's Docker
	// container should be cleaned up once rendering is done.
	AnnotationKeyRuntimeDockerCleanup = "render.crossplane.io/runtime-docker-cleanup"

	// AnnotationKeyRuntimeDockerImage overrides the Docker image that will be
	// used to run the Function. By default render assumes the Function package
	// (i.e. spec.package) can be used to run the Function.
	AnnotationKeyRuntimeDockerImage = "render.crossplane.io/runtime-docker-image"
)

// DockerCleanup specifies what Docker should do with a Function container after
// it has been run.
type DockerCleanup string

// Supported AnnotationKeyRuntimeDockerCleanup values.
const (
	// AnnotationValueRuntimeDockerCleanupStop is the default. It stops the
	// container once rendering is done.
	AnnotationValueRuntimeDockerCleanupStop DockerCleanup = "Stop"

	// AnnotationValueRuntimeDockerCleanupOrphan leaves the container running
	// once rendering is done.
	AnnotationValueRuntimeDockerCleanupOrphan DockerCleanup = "Orphan"

	AnnotationValueRuntimeDockerCleanupDefault = AnnotationValueRuntimeDockerCleanupStop
)

// AnnotationKeyRuntimeDockerPullPolicy can be added to a Function to control how its runtime
// image is pulled.
const AnnotationKeyRuntimeDockerPullPolicy = "render.crossplane.io/runtime-docker-pull-policy"

// DockerPullPolicy can be added to a Function to control how its runtime image
// is pulled by Docker.
type DockerPullPolicy string

// Supported pull policies.
const (
	// Always pull the image.
	AnnotationValueRuntimeDockerPullPolicyAlways DockerPullPolicy = "Always"

	// Never pull the image.
	AnnotationValueRuntimeDockerPullPolicyNever DockerPullPolicy = "Never"

	// Pull the image if it's not present.
	AnnotationValueRuntimeDockerPullPolicyIfNotPresent DockerPullPolicy = "IfNotPresent"

	AnnotationValueRuntimeDockerPullPolicyDefault DockerPullPolicy = AnnotationValueRuntimeDockerPullPolicyIfNotPresent
)

// RuntimeDocker uses a Docker daemon to run a Function.
type RuntimeDocker struct {
	// Image to run
	Image string

	// Stop container once rendering is done
	Stop bool

	// PullPolicy controls how the runtime image is pulled.
	PullPolicy DockerPullPolicy
}

// GetDockerPullPolicy extracts PullPolicy configuration from the supplied
// Function.
func GetDockerPullPolicy(fn pkgv1beta1.Function) (DockerPullPolicy, error) {
	switch p := DockerPullPolicy(fn.GetAnnotations()[AnnotationKeyRuntimeDockerPullPolicy]); p {
	case AnnotationValueRuntimeDockerPullPolicyAlways, AnnotationValueRuntimeDockerPullPolicyNever, AnnotationValueRuntimeDockerPullPolicyIfNotPresent:
		return p, nil
	case "":
		return AnnotationValueRuntimeDockerPullPolicyDefault, nil
	default:
		return "", errors.Errorf("unsupported %q annotation value %q (unknown pull policy)", AnnotationKeyRuntimeDockerPullPolicy, p)
	}
}

// GetDockerCleanup extracts Cleanup configuration from the supplied Function.
func GetDockerCleanup(fn pkgv1beta1.Function) (DockerCleanup, error) {
	switch c := DockerCleanup(fn.GetAnnotations()[AnnotationKeyRuntimeDockerCleanup]); c {
	case AnnotationValueRuntimeDockerCleanupStop, AnnotationValueRuntimeDockerCleanupOrphan:
		return c, nil
	case "":
		return AnnotationValueRuntimeDockerCleanupDefault, nil
	default:
		return "", errors.Errorf("unsupported %q annotation value %q (unknown cleanup policy)", AnnotationKeyRuntimeDockerCleanup, c)
	}
}

// GetRuntimeDocker extracts RuntimeDocker configuration from the supplied
// Function.
func GetRuntimeDocker(fn pkgv1beta1.Function) (*RuntimeDocker, error) {
	cleanup, err := GetDockerCleanup(fn)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get cleanup policy for Function %q", fn.GetName())
	}
	// TODO(negz): Pull package in case it has a different controller image? I
	// hope in most cases Functions will use 'fat' packages, and it's possible
	// to manually override with an annotation so maybe not worth it.
	pullPolicy, err := GetDockerPullPolicy(fn)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get pull policy for Function %q", fn.GetName())
	}
	r := &RuntimeDocker{
		Image:      fn.Spec.Package,
		Stop:       cleanup == AnnotationValueRuntimeDockerCleanupStop,
		PullPolicy: pullPolicy,
	}
	if i := fn.GetAnnotations()[AnnotationKeyRuntimeDockerImage]; i != "" {
		r.Image = i
	}
	return r, nil
}

var _ Runtime = &RuntimeDocker{}

// Start a Function as a Docker container.
func (r *RuntimeDocker) Start(ctx context.Context) (RuntimeContext, error) { //nolint:gocyclo // TODO(phisco): Refactor to break this up a bit, not so easy.
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return RuntimeContext{}, errors.Wrap(err, "cannot create Docker client using environment variables")
	}

	// Find a random, available port. There's a chance of a race here, where
	// something else binds to the port before we start our container.
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return RuntimeContext{}, errors.Wrap(err, "cannot get available TCP port")
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	spec := fmt.Sprintf("%s:9443/tcp", addr)
	expose, bind, err := nat.ParsePortSpecs([]string{spec})
	if err != nil {
		return RuntimeContext{}, errors.Wrapf(err, "cannot parse Docker port spec %q", spec)
	}

	cfg := &container.Config{
		Image:        r.Image,
		Cmd:          []string{"--insecure"},
		ExposedPorts: expose,
	}
	hcfg := &container.HostConfig{
		PortBindings: bind,
	}

	if r.PullPolicy == AnnotationValueRuntimeDockerPullPolicyAlways {
		err = PullImage(ctx, c, r.Image)
		if err != nil {
			return RuntimeContext{}, errors.Wrapf(err, "cannot pull Docker image %q", r.Image)
		}
	}

	// TODO(negz): Set a container name? Presumably unique across runs.
	rsp, err := c.ContainerCreate(ctx, cfg, hcfg, nil, nil, "")
	if err != nil {
		if !errdefs.IsNotFound(err) || r.PullPolicy == AnnotationValueRuntimeDockerPullPolicyNever {
			return RuntimeContext{}, errors.Wrap(err, "cannot create Docker container")
		}

		// The image was not found, but we're allowed to pull it.
		err = PullImage(ctx, c, r.Image)
		if err != nil {
			return RuntimeContext{}, errors.Wrapf(err, "cannot pull Docker image %q", r.Image)
		}

		rsp, err = c.ContainerCreate(ctx, cfg, hcfg, nil, nil, "")
		if err != nil {
			return RuntimeContext{}, errors.Wrap(err, "cannot create Docker container")
		}
	}

	if err := c.ContainerStart(ctx, rsp.ID, container.StartOptions{}); err != nil {
		return RuntimeContext{}, errors.Wrap(err, "cannot start Docker container")
	}

	stop := func(_ context.Context) error {
		// TODO(negz): Maybe log to stderr that we're leaving the container running?
		return nil
	}
	if r.Stop {
		stop = func(ctx context.Context) error {
			err := c.ContainerStop(ctx, rsp.ID, container.StopOptions{})
			return errors.Wrap(err, "cannot stop Docker container")
		}
	}

	return RuntimeContext{Target: addr, Stop: stop}, nil
}

type pullClient interface {
	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
}

// PullImage pulls the supplied image using the supplied client. It blocks until
// the image has either finished pulling or hit an error.
func PullImage(ctx context.Context, p pullClient, image string) error {
	out, err := p.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck // TODO(negz): Can this error?

	// Each line read from out is a JSON object containing the status of the
	// pull - similar to the progress bars you'd see if running docker pull. It
	// seems that consuming all of this output is the best way to block until
	// the image is actually pulled before we try to run it.
	_, err = io.Copy(io.Discard, out)
	return err
}

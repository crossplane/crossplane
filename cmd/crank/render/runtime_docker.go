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
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	typesimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
)

// FunctionPort is the port that Composition Functions listen on inside their container.
const FunctionPort = 9443

// Annotations that can be used to configure the Docker runtime.
const (
	// AnnotationKeyRuntimeDockerCleanup configures how a Function's Docker
	// container should be cleaned up once rendering is done.
	AnnotationKeyRuntimeDockerCleanup = "render.crossplane.io/runtime-docker-cleanup"

	// AnnotationKeyRuntimeDockerImage overrides the Docker image that will be
	// used to run the Function. By default render assumes the Function package
	// (i.e. spec.package) can be used to run the Function.
	AnnotationKeyRuntimeDockerImage = "render.crossplane.io/runtime-docker-image"

	// AnnotationKeyRuntimeNamedContainer sets the Docker container name that will
	// be used for the container. it will also reuse the same container as long as
	// it is available and also try to restart if it is not running.
	AnnotationKeyRuntimeNamedContainer = "render.crossplane.io/runtime-docker-name"

	// AnnotationKeyRuntimeEnvironmentVariables sets the environment variables
	// that will be used for the container. This is helpful to control kpm registry
	// access to use a different registry.
	// It is a comma separated string of key=value pairs e.g. "key1=value1,key2=value2".
	AnnotationKeyRuntimeEnvironmentVariables = "render.crossplane.io/runtime-docker-env"

	// AnnotationKeyRuntimeDockerPublishAddress configures the host address that
	// Docker should publish (bind) the Function's container port to. Defaults to 127.0.0.1.
	// Use 0.0.0.0 to publish to all host interfaces for remote Docker access.
	AnnotationKeyRuntimeDockerPublishAddress = "render.crossplane.io/runtime-docker-publish-address"

	// AnnotationKeyRuntimeDockerTarget configures the address that the render
	// CLI should use to connect to the Function's Docker container.
	AnnotationKeyRuntimeDockerTarget = "render.crossplane.io/runtime-docker-target"
)

// DockerCleanup specifies what Docker should do with a Function container after
// it has been run.
type DockerCleanup string

// Supported AnnotationKeyRuntimeDockerCleanup values.
const (
	// AnnotationValueRuntimeDockerCleanupStop is the default. It stops the
	// container once rendering is done.
	AnnotationValueRuntimeDockerCleanupStop DockerCleanup = "Stop"

	// AnnotationValueRuntimeDockerCleanupRemove stops and removes the
	// container once rendering is done.
	AnnotationValueRuntimeDockerCleanupRemove DockerCleanup = "Remove"

	// AnnotationValueRuntimeDockerCleanupOrphan leaves the container running
	// once rendering is done.
	AnnotationValueRuntimeDockerCleanupOrphan DockerCleanup = "Orphan"

	AnnotationValueRuntimeDockerCleanupDefault = AnnotationValueRuntimeDockerCleanupRemove
)

// AnnotationKeyRuntimeDockerPullPolicy can be added to a Function to control how its runtime
// image is pulled.
const AnnotationKeyRuntimeDockerPullPolicy = "render.crossplane.io/runtime-docker-pull-policy"

// DockerPullPolicy can be added to a Function to control how its runtime image
// is pulled by Docker.
type DockerPullPolicy string

// Supported pull policies.
const (
	// AnnotationValueRuntimeDockerPullPolicyAlways always pulls the image.
	AnnotationValueRuntimeDockerPullPolicyAlways DockerPullPolicy = "Always"

	// AnnotationValueRuntimeDockerPullPolicyNever never pulls the image.
	AnnotationValueRuntimeDockerPullPolicyNever DockerPullPolicy = "Never"

	// AnnotationValueRuntimeDockerPullPolicyIfNotPresent pulls the image if
	// it's not present.
	AnnotationValueRuntimeDockerPullPolicyIfNotPresent DockerPullPolicy = "IfNotPresent"

	AnnotationValueRuntimeDockerPullPolicyDefault DockerPullPolicy = AnnotationValueRuntimeDockerPullPolicyIfNotPresent
)

// RuntimeDocker uses a Docker daemon to run a Function.
type RuntimeDocker struct {
	// Image to run
	Image string

	// Container name
	Name string

	// Cleanup controls how the containers are handled after rendering.
	Cleanup DockerCleanup

	// PullPolicy controls how the runtime image is pulled.
	PullPolicy DockerPullPolicy

	// Keychain to use for pulling images from private registry.
	Keychain authn.Keychain

	// log is the logger for this runtime.
	log logging.Logger

	// Env is the list of environment variables to set for the container.
	Env []string

	// BindAddress is the address to bind the function container to.
	BindAddress string

	// Target is the host address to use when connecting to the function.
	// If empty, it defaults to the published HostIP from Docker inspect.
	// When published on 0.0.0.0, set this explicitly (e.g. the remote Docker host).
	Target string
}

// GetDockerPullPolicy extracts PullPolicy configuration from the supplied
// Function.
func GetDockerPullPolicy(fn pkgv1.Function) (DockerPullPolicy, error) {
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
func GetDockerCleanup(fn pkgv1.Function) (DockerCleanup, error) {
	switch c := DockerCleanup(fn.GetAnnotations()[AnnotationKeyRuntimeDockerCleanup]); c {
	case AnnotationValueRuntimeDockerCleanupStop, AnnotationValueRuntimeDockerCleanupOrphan, AnnotationValueRuntimeDockerCleanupRemove:
		return c, nil
	case "":
		return AnnotationValueRuntimeDockerCleanupDefault, nil
	default:
		return "", errors.Errorf("unsupported %q annotation value %q (unknown cleanup policy)", AnnotationKeyRuntimeDockerCleanup, c)
	}
}

// GetRuntimeDocker extracts RuntimeDocker configuration from the supplied
// Function.
func GetRuntimeDocker(fn pkgv1.Function, log logging.Logger) (*RuntimeDocker, error) {
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
		Image:       fn.Spec.Package,
		Name:        "",
		Cleanup:     cleanup,
		PullPolicy:  pullPolicy,
		Keychain:    authn.DefaultKeychain,
		log:         log,
		BindAddress: "127.0.0.1", // Default to localhost for security
	}

	if i := fn.GetAnnotations()[AnnotationKeyRuntimeDockerImage]; i != "" {
		r.Image = i
	}

	if i := fn.GetAnnotations()[AnnotationKeyRuntimeNamedContainer]; i != "" {
		r.Name = i
	}

	if i := fn.GetAnnotations()[AnnotationKeyRuntimeEnvironmentVariables]; i != "" {
		pairs := strings.SplitSeq(i, ",")
		for pair := range pairs {
			if !strings.Contains(pair, "=") {
				r.log.Debug("ignoring invalid environment variable", "pair", pair)
				continue
			}

			r.Env = append(r.Env, pair)
		}
	}

	if i := fn.GetAnnotations()[AnnotationKeyRuntimeDockerPublishAddress]; i != "" {
		r.BindAddress = i
	}

	if i := fn.GetAnnotations()[AnnotationKeyRuntimeDockerTarget]; i != "" {
		r.Target = i
	}

	return r, nil
}

var _ Runtime = &RuntimeDocker{}

func (r *RuntimeDocker) findContainer(ctx context.Context, cli *client.Client) (string, error) {
	if r.Name == "" {
		return "", nil
	}

	inspect, err := cli.ContainerInspect(ctx, r.Name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return "", nil // Container doesn't exist, but that's not an error
		}
		return "", errors.Wrapf(err, "cannot inspect Docker container %q", r.Name)
	}

	return inspect.ID, nil
}

func (r *RuntimeDocker) createContainer(ctx context.Context, cli *client.Client) (string, error) {
	r.log.Debug("Starting Docker container runtime setup", "image", r.Image)

	// Let Docker automatically allocate an available port on the bind address.
	// This avoids race conditions and works reliably with Docker daemons.
	port := nat.Port(fmt.Sprintf("%d/tcp", FunctionPort))

	cfg := &container.Config{
		Image:        r.Image,
		Cmd:          []string{"--insecure"},
		ExposedPorts: nat.PortSet{port: struct{}{}},
		Env:          r.Env,
	}
	hcfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			port: []nat.PortBinding{{
				HostIP:   r.BindAddress,
				HostPort: "0", // "0" => engine allocates an ephemeral port
			}},
		},
	}

	options, err := r.getPullOptions()
	if err != nil {
		// We can continue to pull an image if we don't have the PullOptions with RegistryAuth
		// as long as the image is from a public registry. Therefore, we log the error message and continue.
		r.log.Info("Cannot get pull options", "image", r.Image, "err", err)
	}

	if r.PullPolicy == AnnotationValueRuntimeDockerPullPolicyAlways {
		r.log.Debug("Pulling image with pullPolicy: Always", "image", r.Image)

		err = PullImage(ctx, cli, r.Image, options)
		if err != nil {
			return "", errors.Wrapf(err, "cannot pull Docker image %q", r.Image)
		}
	}

	r.log.Debug("Creating Docker container", "image", r.Image, "name", r.Name)

	rsp, err := cli.ContainerCreate(ctx, cfg, hcfg, nil, nil, r.Name)
	if err != nil {
		if !errdefs.IsNotFound(err) || r.PullPolicy == AnnotationValueRuntimeDockerPullPolicyNever {
			return "", errors.Wrap(err, "cannot create Docker container")
		}

		// The image was not found, but we're allowed to pull it.
		r.log.Debug("Image not found, pulling", "image", r.Image)

		err = PullImage(ctx, cli, r.Image, options)
		if err != nil {
			return "", errors.Wrapf(err, "cannot pull Docker image %q", r.Image)
		}

		rsp, err = cli.ContainerCreate(ctx, cfg, hcfg, nil, nil, r.Name)
		if err != nil {
			return "", errors.Wrap(err, "cannot create Docker container")
		}
	}

	return rsp.ID, nil
}

// startContainer ensures the container is running and returns its address.
func (r *RuntimeDocker) startContainer(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	// Start the container (idempotent - safe to call on running containers)
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", errors.Wrap(err, "cannot start Docker container")
	}

	// Inspect the container to get the actual allocated port
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", errors.Wrap(err, "cannot inspect Docker container")
	}

	// Look up the specific function port instead of taking the first one
	p := nat.Port(fmt.Sprintf("%d/tcp", FunctionPort))
	if len(inspect.NetworkSettings.Ports[p]) == 0 {
		return "", errors.Errorf("container %q has no published binding for port %s", r.Name, p.Port())
	}

	binding := inspect.NetworkSettings.Ports[p][0]
	host := r.Target
	if host == "" {
		host = binding.HostIP
	}
	if host == "" {
		return "", errors.Errorf("container %q has port binding for %s but no host address", r.Name, p.Port())
	}

	return net.JoinHostPort(host, binding.HostPort), nil
}

func (r *RuntimeDocker) getPullOptions() (typesimage.PullOptions, error) {
	// Resolve auth token by looking into keychain
	ref, err := name.ParseReference(r.Image)
	if err != nil {
		return typesimage.PullOptions{}, errors.Wrapf(err, "Image is not a valid reference %s", r.Image)
	}

	auth, err := r.Keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return typesimage.PullOptions{}, errors.Wrapf(err, "Cannot resolve auth for %s", ref.Context().RegistryStr())
	}

	authConfig, err := auth.Authorization()
	if err != nil {
		return typesimage.PullOptions{}, errors.Wrapf(err, "Cannot get auth config for %s", ref.Context().RegistryStr())
	}

	token, err := authConfig.MarshalJSON()
	if err != nil {
		return typesimage.PullOptions{}, errors.Wrapf(err, "Cannot marshal auth config for %s", ref.Context().RegistryStr())
	}

	return typesimage.PullOptions{
		RegistryAuth: base64.StdEncoding.EncodeToString(token),
	}, nil
}

// Start a Function as a Docker container.
func (r *RuntimeDocker) Start(ctx context.Context) (RuntimeContext, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return RuntimeContext{}, errors.Wrap(err, "cannot create Docker client using environment variables")
	}

	// Try to find an existing container with the supplied container name.
	containerID, err := r.findContainer(ctx, cli)
	if err != nil {
		return RuntimeContext{}, err
	}

	// Create new container if not found.
	if containerID == "" {
		containerID, err = r.createContainer(ctx, cli)
		if err != nil {
			return RuntimeContext{}, err
		}
	}

	containerAddr, err := r.startContainer(ctx, cli, containerID)
	if err != nil {
		return RuntimeContext{}, err
	}

	// Inline stop function
	stop := func(ctx context.Context) error {
		switch r.Cleanup {
		case AnnotationValueRuntimeDockerCleanupOrphan:
			return nil
		case AnnotationValueRuntimeDockerCleanupStop:
			if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
				return errors.Wrap(err, "cannot stop Docker container")
			}
		case AnnotationValueRuntimeDockerCleanupRemove:
			if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
				return errors.Wrap(err, "cannot stop Docker container")
			}
			if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
				return errors.Wrap(err, "cannot remove Docker container")
			}
		}

		return nil
	}

	return RuntimeContext{Target: containerAddr, Stop: stop}, nil
}

type pullClient interface {
	ImagePull(ctx context.Context, ref string, options typesimage.PullOptions) (io.ReadCloser, error)
}

// PullImage pulls the supplied image using the supplied client. It blocks until
// the image has either finished pulling or hit an error.
func PullImage(ctx context.Context, p pullClient, image string, options typesimage.PullOptions) error {
	out, err := p.ImagePull(ctx, image, options)
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

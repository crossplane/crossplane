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

// Package docker contains helpers for working with Docker-compatible container
// runtimes.
package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Check attempts to connect to the local Docker daemon and returns an error if
// it's unable to do so.
func Check(ctx context.Context) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}
	if _, err := cli.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping docker daemon")
	}
	return nil
}

// GetContainerIDByName returns the ID of the container with the given name.
func GetContainerIDByName(ctx context.Context, name string, includeStopped bool) (string, bool, error) {
	c, found, err := GetContainerByName(ctx, name, includeStopped)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return c.ID, true, nil
}

// GetContainerByName returns the container with the given name.
func GetContainerByName(ctx context.Context, name string, includeStopped bool) (*container.Summary, bool, error) {
	cli, err := NewClient()
	if err != nil {
		return nil, false, err
	}

	cs, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: name}),
		All:     includeStopped,
	})
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list containers")
	}
	if len(cs) == 0 {
		return nil, false, nil
	}
	return &cs[0], true, nil
}

// GetNetworkIDByName returns the ID of the network with the given name.
func GetNetworkIDByName(ctx context.Context, name string) (string, bool, error) {
	cli, err := NewClient()
	if err != nil {
		return "", false, err
	}

	ns, err := cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: name}),
	})
	if err != nil {
		return "", false, errors.Wrap(err, "failed to list networks")
	}
	if len(ns) == 0 {
		return "", false, nil
	}
	return ns[0].ID, true, nil
}

// StartContainer starts a container with the given name using the given image.
func StartContainer(ctx context.Context, name, img string, opts ...StartContainerOption) (string, error) {
	cfg := &startContainerConfig{
		containerConfig: &container.Config{
			Image: img,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	cli, err := NewClient()
	if err != nil {
		return "", err
	}

	if _, err := cli.ImageInspect(ctx, img); err != nil {
		auth, err := defaultRegistryAuth(img)
		if err != nil {
			return "", err
		}

		out, err := cli.ImagePull(ctx, img, image.PullOptions{
			RegistryAuth: auth,
		})
		if err != nil {
			return "", errors.Wrapf(err, "failed to pull image %q", img)
		}

		if _, err := io.Copy(io.Discard, out); err != nil {
			return "", errors.Wrapf(err, "failed to read image pull output for %s", img)
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		cfg.containerConfig,
		cfg.hostConfig,
		nil,
		nil,
		name,
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to create container")
	}

	for path, tarball := range cfg.copyFiles {
		if err := cli.CopyToContainer(ctx, resp.ID, filepath.Clean(path), bytes.NewReader(tarball), container.CopyToContainerOptions{}); err != nil {
			return "", errors.Wrapf(err, "failed to copy files to container path %s", path)
		}
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", errors.Wrap(err, "failed to start container")
	}

	for _, nid := range cfg.networks {
		if err := cli.NetworkConnect(ctx, nid, resp.ID, nil); err != nil {
			return "", errors.Wrapf(err, "failed to connect container to network %q", nid)
		}
	}

	return resp.ID, nil
}

func defaultRegistryAuth(imageName string) (string, error) {
	hostname, err := resolveRegistryFromImage(imageName)
	if err != nil {
		return "", errors.Wrapf(err, "cannot resolve registry for image %q", imageName)
	}

	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", errors.Wrap(err, "cannot load Docker registry auth config")
	}

	auth, err := cfg.GetAuthConfig(hostname)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get auth config for registry %q", hostname)
	}

	data, err := json.Marshal(auth)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal Docker auth config")
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func resolveRegistryFromImage(img string) (string, error) {
	ref, err := name.ParseReference(img, name.StrictValidation)
	if err != nil {
		return "", errors.Wrapf(err, "cannot parse image reference %q", img)
	}

	return ref.Context().RegistryStr(), nil
}

// StartContainerByID starts an existing container by ID.
func StartContainerByID(ctx context.Context, id string) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}
	return errors.Wrap(cli.ContainerStart(ctx, id, container.StartOptions{}), "failed to start container")
}

type startContainerConfig struct {
	containerConfig *container.Config
	hostConfig      *container.HostConfig
	networks        []string
	copyFiles       map[string][]byte
}

// StartContainerOption provides optional options for StartContainer.
type StartContainerOption func(*startContainerConfig)

// StartWithCommand sets the command to use when starting a container.
func StartWithCommand(cmd []string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		if cfg.containerConfig == nil {
			cfg.containerConfig = &container.Config{}
		}
		cfg.containerConfig.Cmd = cmd
	}
}

// StartWithBindMount adds a bind mount when starting a container.
func StartWithBindMount(hostPath, containerPath string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		if cfg.hostConfig == nil {
			cfg.hostConfig = &container.HostConfig{}
		}
		cfg.hostConfig.Binds = append(cfg.hostConfig.Binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
}

// StartWithNetworkID adds a network to which a container should be added.
func StartWithNetworkID(nid string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		cfg.networks = append(cfg.networks, nid)
	}
}

// StartWithCopyFiles adds files that should be copied to the given path before
// starting the container.
func StartWithCopyFiles(tarball []byte, path string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		if cfg.copyFiles == nil {
			cfg.copyFiles = make(map[string][]byte)
		}
		cfg.copyFiles[path] = tarball
	}
}

// StartWithEnv adds environment variables that will be passed to the container.
func StartWithEnv(env ...string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		cfg.containerConfig.Env = append(cfg.containerConfig.Env, env...)
	}
}

// StartWithWorkingDirectory sets the working directory for the container.
func StartWithWorkingDirectory(path string) StartContainerOption {
	return func(cfg *startContainerConfig) {
		cfg.containerConfig.WorkingDir = path
	}
}

// StopContainerByID stops and removes a container.
func StopContainerByID(ctx context.Context, cid string) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}

	if err := cli.ContainerStop(ctx, cid, container.StopOptions{}); err != nil {
		return errors.Wrap(err, "failed to stop container")
	}
	return errors.Wrap(
		cli.ContainerRemove(ctx, cid, container.RemoveOptions{Force: true, RemoveVolumes: true}),
		"failed to remove container",
	)
}

// WaitForContainerByID waits for the container with the given ID to stop.
func WaitForContainerByID(ctx context.Context, cid string) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}

	statusCh, errCh := cli.ContainerWait(ctx, cid, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		if status.StatusCode != 0 {
			out, err := cli.ContainerLogs(ctx, cid, container.LogsOptions{ShowStdout: true, ShowStderr: true})
			if err != nil {
				return errors.Wrapf(err, "failed to get container logs")
			}

			logs := new(strings.Builder)
			if _, err := io.Copy(logs, out); err != nil {
				return errors.Wrapf(err, "failed to read container logs")
			}

			return fmt.Errorf("container exited with non-zero status: %d, logs: %s", status.StatusCode, logs.String())
		}
	case err := <-errCh:
		return errors.Wrapf(err, "container unknown failure")
	}

	return nil
}

// RunContainerOption provides optional options for RunContainer.
type RunContainerOption func(*runContainerConfig)

type runContainerConfig struct {
	containerConfig *container.Config
	hostConfig      *container.HostConfig
	networkConfig   *network.NetworkingConfig
	stdin           []byte
}

// RunWithCommand sets the command to run in the container.
func RunWithCommand(cmd []string) RunContainerOption {
	return func(cfg *runContainerConfig) {
		cfg.containerConfig.Cmd = cmd
	}
}

// RunWithStdin provides data to write to the container's stdin. The container
// is configured with OpenStdin and StdinOnce so it receives EOF after the data
// is written.
func RunWithStdin(data []byte) RunContainerOption {
	return func(cfg *runContainerConfig) {
		cfg.stdin = data
		cfg.containerConfig.OpenStdin = true
		cfg.containerConfig.StdinOnce = true
		cfg.containerConfig.AttachStdin = true
	}
}

// RunWithNetworkName connects the container to a Docker network by name.
func RunWithNetworkName(name string) RunContainerOption {
	return func(cfg *runContainerConfig) {
		if cfg.networkConfig == nil {
			cfg.networkConfig = &network.NetworkingConfig{}
		}
		if cfg.networkConfig.EndpointsConfig == nil {
			cfg.networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
		}
		cfg.networkConfig.EndpointsConfig[name] = &network.EndpointSettings{}
	}
}

// RunWithExtraHosts adds extra /etc/hosts entries to the container (e.g.
// "host.docker.internal:host-gateway").
func RunWithExtraHosts(hosts []string) RunContainerOption {
	return func(cfg *runContainerConfig) {
		cfg.hostConfig.ExtraHosts = append(cfg.hostConfig.ExtraHosts, hosts...)
	}
}

// RunWithBindMount adds a bind mount to the container.
func RunWithBindMount(hostPath, containerPath string) RunContainerOption {
	return func(cfg *runContainerConfig) {
		cfg.hostConfig.Binds = append(cfg.hostConfig.Binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
}

// RunContainer creates a container, optionally pipes stdin, waits for it to
// exit, and returns stdout and stderr. The container is always removed on
// return. This is intended for short-lived "run to completion" containers.
func RunContainer(ctx context.Context, img string, opts ...RunContainerOption) ([]byte, []byte, error) {
	cfg := &runContainerConfig{
		containerConfig: &container.Config{
			Image:        img,
			AttachStdout: true,
			AttachStderr: true,
		},
		hostConfig: &container.HostConfig{},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	cli, err := NewClient()
	if err != nil {
		return nil, nil, err
	}

	// Pull the image if it's not already present.
	if _, err := cli.ImageInspect(ctx, img); err != nil {
		auth, authErr := defaultRegistryAuth(img)
		if authErr != nil {
			return nil, nil, authErr
		}
		out, pullErr := cli.ImagePull(ctx, img, image.PullOptions{RegistryAuth: auth})
		if pullErr != nil {
			return nil, nil, errors.Wrapf(pullErr, "failed to pull image %q", img)
		}
		if _, err := io.Copy(io.Discard, out); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to read image pull output for %q", img)
		}
	}

	resp, err := cli.ContainerCreate(ctx, cfg.containerConfig, cfg.hostConfig, cfg.networkConfig, nil, "")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create container")
	}
	defer func() { //nolint:contextcheck // Intentionally use a detached context for cleanup.
		_ = cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	}()

	// Attach before starting so we don't miss any output. Docker
	// multiplexes stdout/stderr with 8-byte frame headers when the
	// container is not using a TTY.
	attach, err := cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
		Stdin:  cfg.stdin != nil,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to attach to container")
	}
	defer attach.Close()

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start container")
	}

	// Write stdin data if provided, then close the write side so the
	// container sees EOF.
	if cfg.stdin != nil {
		if _, err := attach.Conn.Write(cfg.stdin); err != nil {
			return nil, nil, errors.Wrap(err, "failed to write to container stdin")
		}
		if err := attach.CloseWrite(); err != nil {
			return nil, nil, errors.Wrap(err, "failed to close container stdin")
		}
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attach.Reader); err != nil {
		return nil, nil, errors.Wrap(err, "failed to read container output")
	}

	// Wait for the container to finish.
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("container exited with status %d: %s", status.StatusCode, stderr.String())
		}
	case err := <-errCh:
		return nil, nil, errors.Wrap(err, "error waiting for container")
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

// NewClient creates a new Docker client configured from environment variables.
func NewClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create docker client")
	}
	return cli, nil
}

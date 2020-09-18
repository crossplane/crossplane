/*
Copyright 2020 The Crossplane Authors.

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

package pkg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane/pkg/controller/pkg/revision"
)

// LintCmd uses the crossplane package controller linter to validate the contents of a package.
// The rules for linting different kinds of packages (configuration, provider) are determined
// by the package type's subcommand, so the *revision.PackageLinter is passed in by the calling cmd
type LintCmd struct {
	Image  string `arg:"" optional:"" name:"image" help:"Name of the package image to run and lint the output of."`
	NoPull bool   `help:"Flag to disable pulling the package image before running it."`
}

// Lint uses docker to run a container using LintCmd.Image and passes stdout into the
// provided revision.PackageLinter Lint() method to give feedback on any linting issues.
// Currently Lint() only yields one error and does not differentiate between warnings and errors.
// Lint it will attempt to pull the specified image using the appropriate docker credentials
// unless the NoPull option is set to true.
func (c *LintCmd) Lint(ctx context.Context, docker *client.Client, linter revision.Linter) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if docker == nil {
		var err error
		docker, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return err
		}
	}
	err := c.dockerPull(ctx, docker)
	if err != nil {
		return err
	}

	outBuf, _, err := c.runContainerGetLogs(ctx, docker)
	if err != nil {
		return err
	}

	metaScheme, _ := revision.BuildMetaScheme()
	objScheme, _ := revision.BuildObjectScheme()
	p := parser.New(metaScheme, objScheme)
	pkg, err := p.Parse(ctx, outBuf)
	if err != nil {
		return err
	}
	err = linter.Lint(pkg)
	if err != nil {
		return err
	}

	fmt.Println("\nNo errors reported by Crossplane package linter!")
	return nil
}

func (c *LintCmd) dockerPull(ctx context.Context, docker *client.Client) error {
	if c.NoPull {
		return nil
	}
	dra, err := NewDockerRegistryAuth(ctx, c.Image)
	if err != nil {
		return err
	}
	options, err := dra.BuildPullOpts(ctx)
	if err != nil {
		return err
	}
	resp, err := docker.ImagePull(ctx, dra.NormalizedImageName(), options)
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, resp)
	if err != nil {
		return err
	}
	err = resp.Close()
	if err != nil {
		return err
	}

	return nil
}

func (c *LintCmd) runContainerGetLogs(ctx context.Context, docker *client.Client) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	resp, err := docker.ContainerCreate(ctx, &container.Config{
		Image: c.Image,
		Tty:   false,
	}, nil, nil, nil, "")
	if err != nil {
		return nil, nil, err
	}

	if err := docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, nil, err
	}

	statusCh, errCh := docker.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, nil, err
		}
	case <-statusCh:
	}

	out, err := docker.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return nil, nil, err
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	_, err = stdcopy.StdCopy(outBuf, errBuf, out)
	return ioutil.NopCloser(outBuf), ioutil.NopCloser(errBuf), err
}

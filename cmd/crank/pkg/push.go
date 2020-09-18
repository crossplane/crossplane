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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
)

// PushCmd pushes a package to a registry.
type PushCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Name of the package to be built. Defaults to name in crossplane.yaml."`

	Tag         string `short:"t" help:"Package version tag." default:"latest"`
	Registry    string `short:"r" help:"Package registry." default:"registry.upbound.io"`
	PackageRoot string `short:"f" help:"Path to crossplane.yaml" default:"."`
}

// Run runs the Build command.
func (p *PushCmd) Run() error {
	if p.Name == "" {
		root, err := filepath.Abs(p.PackageRoot)
		if err != nil {
			return err
		}
		yamlPath := filepath.Join(root, "crossplane.yaml")
		p.Name, err = parseNameFromPackageFile(yamlPath)
		if err != nil {
			return err
		}
	}

	imageName := p.fullImageName()
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	pi := &pushImage{
		docker:       docker,
		pkgImageName: imageName,
		logWriter:    os.Stdout,
	}

	err = pi.push(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nPushed package %s to registry as %s\n", p.Name, imageName)

	return nil
}

// TODO (kasey): should probably refactor all the overlapping logic around building the image name into something shared between build/push
func (p *PushCmd) fullImageName() string {
	return fmt.Sprintf("%s/%s:%s", p.Registry, p.Name, p.Tag)
}

type pushImage struct {
	docker       *client.Client
	pkgImageName string
	logWriter    io.Writer
}

// Push invokes docker's /images/{name}/push api
// https://docs.docker.com/engine/api/v1.40/#operation/ImagePush
func (p *pushImage) push(ctx context.Context) error {
	dra, err := NewDockerRegistryAuth(ctx, p.pkgImageName)
	if err != nil {
		return err
	}
	options, err := dra.BuildPushOpts(ctx)
	if err != nil {
		return err
	}
	resp, err := p.docker.ImagePush(ctx, dra.NormalizedImageName(), options)
	if err != nil {
		return err
	}

	_, err = io.Copy(p.logWriter, resp)
	if err != nil {
		return err
	}
	return resp.Close()
}

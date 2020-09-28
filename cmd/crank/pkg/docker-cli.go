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

---

NOTE: in an effort to maintain compatibility with the specific behavior of the docker cli,
significant portions of this source file were copied from various source files in the
github.com/docker/cli repo, which is also under the Apache 2.0 license.
*/

package pkg

import (
	"context"

	"github.com/docker/cli/cli/command"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
)

// DockerRegistryAuth can be used to build options types that are required to set up
// authentication for docker push/pull operations in exactly the same way as the official
// docker cli.
// Mainly based on code from https://github.com/docker/cli/blob/master/cli/command/image/push.go
type DockerRegistryAuth struct {
	normalizedName string
	repoInfo       *registry.RepositoryInfo
	dockerCli      *command.DockerCli
}

// NewDockerRegistryAuth initializes a DockerRegistryAuth.
func NewDockerRegistryAuth(ctx context.Context, imageURL string) (*DockerRegistryAuth, error) {
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return nil, err
	}
	ref, err := reference.ParseNormalizedNamed(imageURL)
	if err != nil {
		return nil, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}

	return &DockerRegistryAuth{
		normalizedName: reference.FamiliarString(ref),
		repoInfo:       repoInfo,
		dockerCli:      dockerCli,
	}, nil
}

// NormalizedImageName is the result of the same string
// processing/normalization steps that the docker cli performs
func (aw *DockerRegistryAuth) NormalizedImageName() string {
	return aw.normalizedName
}

// BuildPullOpts creates a types.ImagePullOptions with authentication behavior identical to docker pull
func (aw *DockerRegistryAuth) BuildPullOpts(ctx context.Context) (types.ImagePullOptions, error) {
	authConfig := command.ResolveAuthConfig(ctx, aw.dockerCli, aw.repoInfo.Index)
	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return types.ImagePullOptions{}, err
	}
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(aw.dockerCli, aw.repoInfo.Index, "pull")
	return types.ImagePullOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}, nil
}

// BuildPushOpts creates a types.ImagePullOptions with authentication behavior identical to docker push
func (aw *DockerRegistryAuth) BuildPushOpts(ctx context.Context) (types.ImagePushOptions, error) {
	authConfig := command.ResolveAuthConfig(ctx, aw.dockerCli, aw.repoInfo.Index)
	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return types.ImagePushOptions{}, err
	}
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(aw.dockerCli, aw.repoInfo.Index, "push")
	return types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}, nil
}

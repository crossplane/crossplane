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
	"fmt"

	"github.com/docker/docker/api/types/network"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/internal/docker"
)

// createRenderNetwork creates a temporary Docker bridge network for render.
// Function containers and the Crossplane render container join this network so
// they can reach each other. Returns the network ID and name.
func createRenderNetwork(ctx context.Context) (string, string, error) {
	cli, err := docker.NewClient()
	if err != nil {
		return "", "", errors.Wrap(err, "cannot create Docker client")
	}

	name := fmt.Sprintf("crossplane-render-%s", rand.String(8))

	resp, err := cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "cannot create Docker network %q", name)
	}

	return resp.ID, name, nil
}

// removeRenderNetwork removes a temporary Docker network.
func removeRenderNetwork(ctx context.Context, networkID string) error {
	cli, err := docker.NewClient()
	if err != nil {
		return errors.Wrap(err, "cannot create Docker client")
	}
	return errors.Wrap(cli.NetworkRemove(ctx, networkID), "cannot remove Docker network")
}

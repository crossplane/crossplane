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
	"bytes"
	"context"
	"os"
	"os/exec"

	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// localRenderEngine executes a local crossplane binary for rendering.
type localRenderEngine struct {
	// BinaryPath is the path to the crossplane binary.
	BinaryPath string
}

func (e *localRenderEngine) CheckContextSupport() error {
	return nil
}

// Setup is a no-op for the local engine. Function containers publish ports to
// localhost, so there's nothing extra to configure.
func (e *localRenderEngine) Setup(_ context.Context, _ []pkgv1.Function) (func(), error) {
	return func() {}, nil
}

// Render marshals the request, runs it through a local binary, and returns
// the response.
func (e *localRenderEngine) Render(ctx context.Context, req *renderv1alpha1.RenderRequest) (*renderv1alpha1.RenderResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal render request")
	}

	cmd := exec.CommandContext(ctx, e.BinaryPath, "internal", "render") //nolint:gosec // The binary path is user-supplied via CLI flag.
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "cannot run crossplane internal render")
	}

	rsp := &renderv1alpha1.RenderResponse{}
	if err := proto.Unmarshal(out, rsp); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal render response")
	}

	return rsp, nil
}

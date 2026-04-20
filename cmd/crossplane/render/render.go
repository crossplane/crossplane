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

// Package render implements the 'crossplane internal render' subcommand. It
// reads a protobuf RenderRequest from stdin, dispatches to the appropriate
// render implementation based on the oneof variant, and writes a protobuf
// RenderResponse to stdout.
package render

import (
	"context"
	"io"
	"os"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/render/composite"
	"github.com/crossplane/crossplane/v2/internal/render/operation"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Command renders a resource using the real reconciler engine backed by a fake
// in-memory client. It reads a protobuf RenderRequest from stdin and writes a
// protobuf RenderResponse to stdout.
type Command struct {
	Timeout time.Duration `default:"2m" help:"Timeout for the render operation."`
}

// Run executes the render command.
func (c *Command) Run(log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read render request from stdin")
	}

	req := &renderv1alpha1.RenderRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return errors.Wrap(err, "cannot unmarshal render request")
	}

	rsp := &renderv1alpha1.RenderResponse{Meta: &renderv1alpha1.ResponseMeta{}}

	switch in := req.GetInput().(type) {
	case *renderv1alpha1.RenderRequest_Composite:
		out, err := composite.Render(ctx, log, in.Composite)
		if err != nil {
			return errors.Wrap(err, "cannot render composite resource")
		}
		rsp.Output = &renderv1alpha1.RenderResponse_Composite{Composite: out}

	case *renderv1alpha1.RenderRequest_Operation:
		out, err := operation.Render(ctx, log, in.Operation)
		if err != nil {
			return errors.Wrap(err, "cannot render operation")
		}
		rsp.Output = &renderv1alpha1.RenderResponse_Operation{Operation: out}

	case *renderv1alpha1.RenderRequest_CronOperation:
		out, err := operation.NewFromCronOperation(in.CronOperation)
		if err != nil {
			return errors.Wrap(err, "cannot render cron operation")
		}
		rsp.Output = &renderv1alpha1.RenderResponse_CronOperation{CronOperation: out}

	case *renderv1alpha1.RenderRequest_WatchOperation:
		out, err := operation.NewFromWatchOperation(in.WatchOperation)
		if err != nil {
			return errors.Wrap(err, "cannot render watch operation")
		}
		rsp.Output = &renderv1alpha1.RenderResponse_WatchOperation{WatchOperation: out}

	default:
		return errors.New("render request must set exactly one of: composite, operation, cron_operation, watch_operation")
	}

	out, err := proto.Marshal(rsp)
	if err != nil {
		return errors.Wrap(err, "cannot marshal render response")
	}

	_, err = os.Stdout.Write(out)
	return errors.Wrap(err, "cannot write render response")
}

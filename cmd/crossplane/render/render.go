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

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/render/composite"
	"github.com/crossplane/crossplane/v2/internal/render/operation"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// ExitCodePipelineFatal is the process exit code reported when a function
// pipeline step returns a SEVERITY_FATAL result. It is distinct from the
// generic non-zero exit Kong reports for other errors so that wrappers can
// branch on FATAL without parsing stderr. See issue #7446.
const ExitCodePipelineFatal = 3

// Command renders a resource using the real reconciler engine backed by a fake
// in-memory client. It reads a protobuf RenderRequest from stdin and writes a
// protobuf RenderResponse to stdout.
type Command struct {
	Timeout time.Duration `default:"2m" help:"Timeout for the render operation."`

	// stdin and stdout default to os.Stdin/os.Stdout. They are unexported
	// so they don't expand the production API surface; in-package tests can
	// set them to substitute buffers without process-level redirection.
	// AfterApply (called by Kong after parsing) populates these if unset,
	// so Run can use them directly.
	stdin  io.Reader
	stdout io.Writer
}

// AfterApply is invoked by Kong after CLI parsing, before Run. It applies
// stdin/stdout defaults so callers (Kong in production, tests injecting
// buffers) only need to override what they want substituted.
func (c *Command) AfterApply() error {
	if c.stdin == nil {
		c.stdin = os.Stdin
	}
	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	return nil
}

// Run executes the render command.
func (c *Command) Run(log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	data, err := io.ReadAll(c.stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read render request from stdin")
	}

	req := &renderv1alpha1.RenderRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return errors.Wrap(err, "cannot unmarshal render request")
	}

	rsp := &renderv1alpha1.RenderResponse{Meta: &renderv1alpha1.ResponseMeta{}}

	// renderErr captures the render-side failure if any. We resolve it after
	// the switch so the success path can still return a marshalled response,
	// and the pipeline-fatal path can return both the partial response on
	// stdout AND a typed error with a distinct exit code.
	var renderErr error

	switch in := req.GetInput().(type) {
	case *renderv1alpha1.RenderRequest_Composite:
		out, err := composite.Render(ctx, log, in.Composite)
		if out != nil {
			rsp.Output = &renderv1alpha1.RenderResponse_Composite{Composite: out}
		}
		if err != nil {
			renderErr = errors.Wrap(err, "cannot render composite resource")
		}

	case *renderv1alpha1.RenderRequest_Operation:
		out, err := operation.Render(ctx, log, in.Operation)
		if out != nil {
			rsp.Output = &renderv1alpha1.RenderResponse_Operation{Operation: out}
		}
		if err != nil {
			renderErr = errors.Wrap(err, "cannot render operation")
		}

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

	// On a pipeline FATAL we surface the partial output to stdout before
	// propagating the error so callers iterating on requirements can recover
	// the recorded RequiredResources/RequiredSchemas. We only take that
	// path when there's actually a partial output to emit (rsp.Output set):
	// if BuildOutput failed alongside the pipeline FATAL, there's no usable
	// stdout to emit so we fall through to the regular error path. Callers
	// can still recover *PipelineFatalError from the returned error via
	// errors.As regardless of which path we take.
	var pfe *xcomposite.PipelineFatalError
	hasPartialOutput := rsp.GetOutput() != nil && errors.As(renderErr, &pfe)
	if renderErr != nil && !hasPartialOutput {
		return renderErr
	}

	out, err := proto.Marshal(rsp)
	if err != nil {
		// Marshal failure means we cannot deliver any stdout; surface both
		// the marshal error and the pipeline FATAL (if any) via errors.Join
		// so callers can still recover *PipelineFatalError via errors.As.
		// The combined error does not implement kong.ExitCoder, so the
		// process exits with the generic non-zero code instead of 3 — which
		// is correct because the partial-output contract isn't being met.
		merr := errors.Wrap(err, "cannot marshal render response")
		if renderErr != nil {
			return errors.Join(merr, renderErr)
		}
		return merr
	}
	if _, err := c.stdout.Write(out); err != nil {
		werr := errors.Wrap(err, "cannot write render response")
		if renderErr != nil {
			return errors.Join(werr, renderErr)
		}
		return werr
	}

	if pfe != nil {
		// Wrap with an exit-code marker so Kong (via the kong.ExitCoder
		// interface) sets the process exit code to ExitCodePipelineFatal.
		// The wrapped chain still contains *xcomposite.PipelineFatalError,
		// so
		// callers using errors.As can recover it.
		return &exitCodeError{err: renderErr, code: ExitCodePipelineFatal}
	}
	return nil
}

// exitCodeError wraps an error to communicate a specific process exit code
// to Kong via the kong.ExitCoder interface. Unwrap preserves the chain so
// callers using errors.As/Is still see whatever was wrapped.
type exitCodeError struct {
	err  error
	code int
}

func (e *exitCodeError) Error() string { return e.err.Error() }
func (e *exitCodeError) Unwrap() error { return e.err }
func (e *exitCodeError) ExitCode() int { return e.code }

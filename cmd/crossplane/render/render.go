/*
Copyright 2025 The Crossplane Authors.

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

// Package render implements the 'crossplane internal render' subcommand.
package render

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/render/composite"
)

// Command runs one real XR reconcile loop using the real reconciler engine
// backed by a fake in-memory client. It reads composite.Input from stdin and
// writes composite.Output to stdout.
type Command struct {
	Timeout time.Duration `default:"2m" help:"Timeout for the render operation."`
}

// Run executes the render command.
func (c *Command) Run(log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	in, err := readInput(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read render input from stdin")
	}

	out, err := composite.Render(ctx, log, in)
	if err != nil {
		return errors.Wrap(err, "cannot render")
	}

	return writeOutput(os.Stdout, out)
}

// readInput reads and decodes an Input from the reader.
func readInput(r io.Reader) (*composite.Input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read input")
	}

	in := &composite.Input{}
	if err := json.Unmarshal(data, in); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal render input")
	}

	return in, nil
}

// writeOutput encodes and writes an Output to the writer.
func writeOutput(w io.Writer, out *composite.Output) error {
	out.APIVersion = composite.APIVersion
	out.Kind = composite.KindOutput

	data, err := json.Marshal(out)
	if err != nil {
		return errors.Wrap(err, "cannot marshal render output")
	}

	_, err = w.Write(data)
	return errors.Wrap(err, "cannot write render output")
}

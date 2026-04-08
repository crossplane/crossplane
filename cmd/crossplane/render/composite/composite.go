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

// Package composite implements the 'crossplane internal render composite'
// subcommand.
package composite

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

// Command renders a composite resource using the real XR reconciler.
type Command struct {
	Timeout time.Duration `default:"2m" help:"Timeout for the render operation."`
}

// Run reads composite.Input from stdin and writes composite.Output to stdout.
func (c *Command) Run(log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read input")
	}

	in := &composite.Input{}
	if err := json.Unmarshal(data, in); err != nil {
		return errors.Wrap(err, "cannot unmarshal render input")
	}

	out, err := composite.Render(ctx, log, in)
	if err != nil {
		return errors.Wrap(err, "cannot render composite resource")
	}

	out.APIVersion = composite.APIVersion
	out.Kind = composite.KindOutput

	outData, err := json.Marshal(out)
	if err != nil {
		return errors.Wrap(err, "cannot marshal render output")
	}

	_, err = os.Stdout.Write(outData)
	return errors.Wrap(err, "cannot write render output")
}

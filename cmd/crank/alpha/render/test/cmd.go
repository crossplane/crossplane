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

// Package op implements operation rendering using operation functions.
package test

import (
	"context"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// Cmd arguments and flags for alpha render test subcommand.
type Cmd struct {
	// Arguments.

	// Flags. Keep them in alphabetical order.

	Timeout time.Duration `default:"1m" help:"How long to run before timing out."`

	fs afero.Fs
}

// Help prints out the help for the alpha render op command.
func (c *Cmd) Help() string {
	return `
This command renders XRs and asserts the outputs are as expected.

Examples:

  # Run a render test.
  crossplane alpha render test
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run alpha render test.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// Run the test
	_, err := Test(ctx, log, Inputs{})
	if err != nil {
		return err
	}

	return nil
}

/*
Copyright 2023 The Crossplane Authors.

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

// Package version contains version cmd
package version

import (
	"context"
	"flag"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane/internal/version"
)

const (
	errGetCrossplaneVersion = "unable to get crossplane version"
)

// Cmd represents the version command.
type Cmd struct {
	Client bool `env:"" help:"If true, shows client version only (no server required)." json:"client,omitempty"`
}

// BeforeApply sets default values and parses flags.
func (c *Cmd) BeforeApply() error {
	flag.BoolVar(&c.Client, "client", false, "If true, shows client version only (no server required).")
	flag.Parse()
	return nil
}

// Help returns help instructions for the version command.
func (c *Cmd) Help() string {
	return `
Options:
  --client=false:
  If true, shows client version only (no server required).
Usage:
  crossplane version [flags] [options]
`
}

// Run runs the version command.
func (c *Cmd) Run(k *kong.Context) error {
	fmt.Fprintln(k.Stdout, "Client Version: "+version.New().GetVersionString())
	if c.Client {
		return nil
	}

	ctx := context.Background()

	vxp, err := FetchCrossplaneVersion(ctx)
	if err != nil {
		return errors.Wrap(err, errGetCrossplaneVersion)
	}
	if vxp != "" {
		fmt.Fprintln(k.Stdout, "Server Version: "+vxp)
	}

	return nil
}

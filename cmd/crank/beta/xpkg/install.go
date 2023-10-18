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

package xpkg

import (
	"time"

	"github.com/alecthomas/kong"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/xpkg"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type installCmd struct {
	Kind string `arg:"" help:"Kind of package to install. Currently only \"function\" is supported." enum:"function"`
	Ref  string `arg:"" help:"The package's OCI image reference (e.g. tag)."`
	Name string `arg:""  optional:"" help:"Name of the new package. Will be derived from the ref if omitted."`

	Wait                 time.Duration `short:"w" help:"Wait for installation of package"`
	RevisionHistoryLimit int64         `short:"r" help:"Revision history limit."`
	ManualActivation     bool          `short:"m" help:"Enable manual revision activation policy."`
	PackagePullSecrets   []string      `help:"List of secrets used to pull package."`

	Config string `help:"Specify a runtime config."`
}

// Run the package install cmd.
func (c *installCmd) Run(k *kong.Context, logger logging.Logger) error {
	// The beta implementation of this command is identical to the GA one. It
	// exists in beta because Functions are a beta feature, not because the
	// command itself is beta. Wrapping and calling the GA implementation allows
	// us to reuse the code while exposing a command with slightly different
	// semantics. In particular the GA command struct uses an enum struct tag to
	// ensure it only supports providers and configurations, while this beta
	// command only supports functions.
	wrapped := xpkg.InstallCmd{
		Kind:                 c.Kind,
		Ref:                  c.Ref,
		Name:                 c.Name,
		Wait:                 c.Wait,
		RevisionHistoryLimit: c.RevisionHistoryLimit,
		ManualActivation:     c.ManualActivation,
		PackagePullSecrets:   c.PackagePullSecrets,
		Config:               c.Config,
	}
	return wrapped.Run(k, logger)
}

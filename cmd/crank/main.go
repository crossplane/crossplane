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

package main

import (
	"github.com/alecthomas/kong"

	"github.com/crossplane/crossplane/cmd/crank/configuration"
	"github.com/crossplane/crossplane/cmd/crank/pkg"
	"github.com/crossplane/crossplane/cmd/crank/provider"
)

var cli struct {
	Configuration configuration.Cmd `cmd:"" help:"Interact with Configurations."`
	Provider      provider.Cmd      `cmd:"" help:"Interact with Providers."`
	Pkg           pkg.Cmd           `cmd:"" help:"Build and publish packages."`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.Name("crank"),
		kong.Description("A tool for building platforms on Crossplane."),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

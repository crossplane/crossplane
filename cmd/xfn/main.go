/*
Copyright 2019 The Crossplane Authors.

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

// Package main is the reference implementation of Composition Functions.
package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/xfn/run"
	"github.com/crossplane/crossplane/cmd/xfn/spark"
	"github.com/crossplane/crossplane/internal/version"
)

type debugFlag bool
type versionFlag bool

var cli struct {
	Debug debugFlag `short:"d" help:"Print verbose logging statements."`

	Version versionFlag `short:"v" help:"Print version and quit."`

	// Start start.Command `cmd:"" help:"Start Composition Function runner." default:"1"`
	Run   run.Command   `cmd:"" help:"Run a Composition Function."`
	Spark spark.Command `cmd:"" help:"xfn executes Spark inside a user namespace to run a Composition Function. You shouldn't run it directly." hidden:""`
}

// BeforeApply binds the dev mode logger to the kong context when debugFlag is
// passed.
func (d debugFlag) BeforeApply(ctx *kong.Context) error { //nolint:unparam // BeforeApply requires this signature.
	zl := zap.New(zap.UseDevMode(true)).WithName("crossplane")
	// BindTo uses reflect.TypeOf to get reflection type of used interface
	// A *logging.Logger value here is used to find the reflection type here.
	// Please refer: https://golang.org/pkg/reflect/#TypeOf
	ctx.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil))
	return nil
}

func (v versionFlag) BeforeApply(app *kong.Kong) error { //nolint:unparam // BeforeApply requires this signature.
	fmt.Fprintln(app.Stdout, version.New().GetVersionString())
	app.Exit(0)
	return nil
}

func main() {
	zl := zap.New().WithName("xfn")

	ctx := kong.Parse(&cli,
		kong.Name("crossplane"),
		kong.Description("Crossplane Composition Functions."),
		kong.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil)),
		kong.UsageOnError(),
	)
	ctx.FatalIfErrorf(ctx.Run())
}

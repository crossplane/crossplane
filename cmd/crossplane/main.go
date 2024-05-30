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

// Package main implements Crossplane's controller managers.
package main

import (
	"fmt"
	"io"

	"github.com/alecthomas/kong"
	admv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/cmd/crossplane/core"
	"github.com/crossplane/crossplane/cmd/crossplane/rbac"
	"github.com/crossplane/crossplane/internal/version"
)

type (
	debugFlag   bool
	versionFlag bool
)

type cli struct {
	Debug debugFlag `help:"Print verbose logging statements." short:"d"`

	Version versionFlag `help:"Print version and quit." short:"v"`

	Core core.Command `cmd:"" default:"withargs"                                help:"Start core Crossplane controllers."`
	Rbac rbac.Command `cmd:"" help:"Start Crossplane RBAC Manager controllers."`
}

// BeforeApply binds the dev mode logger to the kong context
// when debugFlag is passed.
// This method requires unparam lint exception as Kong expects
// an error value in return from Hook methods but in our case
// there are no error introducing steps.
func (d debugFlag) BeforeApply(ctx *kong.Context) error { //nolint:unparam // BeforeApply requires this signature.
	zl := zap.New(zap.UseDevMode(true)).WithName("crossplane")
	// BindTo uses reflect.TypeOf to get reflection type of used interface
	// A *logging.Logger value here is used to find the reflection type here.
	// Please refer: https://golang.org/pkg/reflect/#TypeOf
	ctx.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil))
	// The controller-runtime runs with a no-op logger by default. It is
	// *very* verbose even at info level, so we only provide it a real
	// logger when we're running in debug mode.
	ctrl.SetLogger(zl)
	logging.SetFilteredKlogLogger(zl)
	return nil
}

func (v versionFlag) BeforeApply(app *kong.Kong) error { //nolint:unparam // BeforeApply requires this signature.
	_, _ = fmt.Fprintln(app.Stdout, version.New().GetVersionString())
	app.Exit(0)
	return nil
}

func main() {
	zl := zap.New().WithName("crossplane")
	logging.SetFilteredKlogLogger(zl)

	// Setting the controller-runtime logger to a no-op logger by default,
	// unless debug mode is enabled. This is because the controller-runtime
	// logger is *very* verbose even at info level. This is not really needed,
	// but otherwise we get a warning from the controller-runtime.
	ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))

	// Note that the controller managers scheme must be a superset of the
	// package manager's object scheme; it must contain all object types that
	// may appear in a Crossplane package. This is because the package manager
	// uses the controller manager's client (and thus scheme) to create packaged
	// objects.
	s := runtime.NewScheme()

	ctx := kong.Parse(&cli{},
		kong.Name("crossplane"),
		kong.Description("An open source multicloud control plane."),
		kong.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil)),
		kong.UsageOnError(),
		rbac.KongVars,
		core.KongVars,
	)
	ctx.FatalIfErrorf(corev1.AddToScheme(s), "cannot add core v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(appsv1.AddToScheme(s), "cannot add apps v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(rbacv1.AddToScheme(s), "cannot add rbac v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(coordinationv1.AddToScheme(s), "cannot add coordination v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(extv1.AddToScheme(s), "cannot add apiextensions v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(extv1beta1.AddToScheme(s), "cannot add apiextensions v1beta1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(admv1.AddToScheme(s), "cannot add admissionregistration v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(apis.AddToScheme(s), "cannot add Crossplane API types to scheme")
	ctx.FatalIfErrorf(ctx.Run(s))
}

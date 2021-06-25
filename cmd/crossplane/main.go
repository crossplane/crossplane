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

package main

import (
	"strings"

	"github.com/alecthomas/kong"
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
)

type debugFlag bool

var cli struct {
	Debug debugFlag `short:"d" help:"Print verbose logging statements."`

	Core core.Command `cmd:"" help:"Start core Crossplane controllers." default:"1"`
	Rbac rbac.Command `cmd:"" help:"Start Crossplane RBAC Manager controllers."`
}

func (d debugFlag) BeforeApply(ctx *kong.Context) error { // nolint:unparam
	zl := zap.New(zap.UseDevMode(true))
	ctx.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil))
	ctrl.SetLogger(zl)
	return nil
}

func main() {
	// // NOTE(negz): We must setup our logger after calling kingpin.MustParse in
	// // order to ensure the debug flag has been parsed and set.
	zl := zap.New(zap.UseDevMode(false))
	ctrl.SetLogger(zl)

	// Note that the controller managers scheme must be a superset of the
	// package manager's object scheme; it must contain all object types that
	// may appear in a Crossplane package. This is because the package manager
	// uses the controller manager's client (and thus scheme) to create packaged
	// objects.
	s := runtime.NewScheme()

	ctx := kong.Parse(&cli,
		kong.Name("crossplane"),
		kong.Description("An open source multicloud control plane."),
		kong.BindTo(logging.NewLogrLogger(zl), (*logging.Logger)(nil)),
		kong.UsageOnError(),
		kong.Vars{
			"rbac_manage_default_var": rbac.ManagementPolicyAll,
			"rbac_manage_enum_var": strings.Join(
				[]string{
					rbac.ManagementPolicyAll,
					rbac.ManagementPolicyBasic,
				},
				", "),
		},
	)
	ctx.FatalIfErrorf(corev1.AddToScheme(s), "cannot add core v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(appsv1.AddToScheme(s), "cannot add apps v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(rbacv1.AddToScheme(s), "cannot add rbac v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(coordinationv1.AddToScheme(s), "cannot add coordination v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(extv1.AddToScheme(s), "cannot add apiextensions v1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(extv1beta1.AddToScheme(s), "cannot add apiextensions v1beta1 Kubernetes API types to scheme")
	ctx.FatalIfErrorf(apis.AddToScheme(s), "cannot add Crossplane API types to scheme")
	err := ctx.Run(s)
	ctx.FatalIfErrorf(err)
}

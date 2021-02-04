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
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane/cmd/crossplane/core"
	"github.com/crossplane/crossplane/cmd/crossplane/rbac"
)

type debugFlag bool

func (d *debugFlag) AfterApply(zl *logr.Logger) error { // nolint:unparam
	*zl = zap.New(zap.UseDevMode(bool(*d)))
	if *d {
		ctrl.SetLogger(*zl)
	}
	return nil
}

var cli struct {
	Debug debugFlag `help:"Enable debug loggging."`

	Core core.Cmd `cmd:"" help:"Start core Crossplane controllers."`
	Rbac rbac.Cmd `cmd:"" help:"Start Crossplane RBAC Manager controllers."`
}

func main() {

	// NOTE(negz): We must setup our logger after calling kingpin.MustParse in
	// order to ensure the debug flag has been parsed and set.
	zl := zap.New(zap.UseDevMode(false))

	ctx := kong.Parse(&cli,
		kong.Name("crossplane"),
		kong.Description("An open source multicloud control plane."),
		kong.Bind(&zl),
		kong.UsageOnError(),
		kong.Vars{
			"rbac_manage_default_var": rbac.ManagementPolicyAll,
			"rbac_manage_enum_var": strings.Join(
				[]string{
					rbac.ManagementPolicyAll,
					rbac.ManagementPolicyBasic,
				},
				", "),
		})
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

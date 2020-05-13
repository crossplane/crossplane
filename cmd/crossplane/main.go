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
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crossplane/core"
	"github.com/crossplane/crossplane/cmd/crossplane/package/manage"
	"github.com/crossplane/crossplane/cmd/crossplane/package/unpack"
)

func main() {
	var (
		app   = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		pkg   = app.Command("package", "Perform operations on packages")
	)

	c := core.FromKingpin(app.Command("core", "Start core Crossplane controllers.").Default())
	m := manage.FromKingpin(pkg.Command("manage", "Start Crossplane Package Manager controllers"))
	u := unpack.FromKingpin(pkg.Command("unpack", "Unpack a Package").Alias("unpackage"))
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	// NOTE(negz): We must setup our logger after calling kingpin.MustParse in
	// order to ensure the debug flag has been parsed and set.
	zl := zap.New(zap.UseDevMode(*debug))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	switch cmd {
	case c.Name:
		kingpin.FatalIfError(c.Run(logging.NewLogrLogger(zl.WithName("crossplane"))), "cannot run crossplane")
	case m.Name:
		kingpin.FatalIfError(m.Run(logging.NewLogrLogger(zl.WithName("package-manager"))), "cannot run package manager")
	case u.Name:
		kingpin.FatalIfError(u.Run(logging.NewLogrLogger(zl.WithName("package-unpack"))), "cannot unpack package")
	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}
}

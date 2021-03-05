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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/cmd/crossplane/core"
	"github.com/crossplane/crossplane/cmd/crossplane/rbac"
)

func main() {
	var (
		app   = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	)
	c, ci := core.FromKingpin(app.Command("core", "Start core Crossplane controllers.").Default())
	r, ri := rbac.FromKingpin(app.Command("rbac", "Start Crossplane RBAC Manager controllers."))
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
	// Note that the controller managers scheme must be a superset of the
	// package manager's object scheme; it must contain all object types that
	// may appear in a Crossplane package. This is because the package manager
	// uses the controller manager's client (and thus scheme) to create packaged
	// objects.
	s := runtime.NewScheme()
	kingpin.FatalIfError(scheme.AddToScheme(s), "cannot add client-go scheme")
	kingpin.FatalIfError(extv1.AddToScheme(s), "cannot add client-go extensions v1 scheme")
	kingpin.FatalIfError(extv1beta1.AddToScheme(s), "cannot add client-go extensions v1beta1 scheme")
	kingpin.FatalIfError(apis.AddToScheme(s), "cannot add crossplane scheme")

	switch cmd {
	case c.Name:
		kingpin.FatalIfError(c.Run(s, logging.NewLogrLogger(zl.WithName("core"))), "cannot run crossplane")
	case ci.Name:
		kingpin.FatalIfError(ci.Run(s, logging.NewLogrLogger(zl.WithName("core init"))), "cannot initialize crossplane")
	case r.Name:
		kingpin.FatalIfError(r.Run(s, logging.NewLogrLogger(zl.WithName("rbac"))), "cannot run RBAC manager")
	case ri.Name:
		kingpin.FatalIfError(ri.Run(s, logging.NewLogrLogger(zl.WithName("rbac init"))), "cannot initialize RBAC manager")
	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}
}

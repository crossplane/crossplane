/*
Copyright 2018 The Crossplane Authors.

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
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/crossplaneio/crossplane/pkg/apis"
	"github.com/crossplaneio/crossplane/pkg/controller"
	"github.com/crossplaneio/crossplane/pkg/log"
)

func main() {
	var (
		logger = log.Log

		app   = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := runtimelog.ZapLogger(*debug)
	log.SetLogger(zl)
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		runtimelog.SetLogger(zl)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		kingpin.FatalIfError(err, "Cannot get config")
	}

	// Re-sync resources every minutes
	// TODO: 1 minute could be too aggressive - we will update this in the future.
	syncPeriod := 1 * time.Minute

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{SyncPeriod: &syncPeriod})
	if err != nil {
		kingpin.FatalIfError(err, "Cannot create manager")
	}

	logger.Info("Adding schemes")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		kingpin.FatalIfError(err, "Cannot add APIs to scheme")
	}

	logger.Info("Adding controllers")

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		kingpin.FatalIfError(err, "Cannot add controllers to manager")
	}

	logger.Info("Starting the manager")

	// Start the Cmd
	kingpin.FatalIfError(mgr.Start(signals.SetupSignalHandler()), "Cannot start controller")
}

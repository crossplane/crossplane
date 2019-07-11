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
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/crossplaneio/crossplane/pkg/apis"
	"github.com/crossplaneio/crossplane/pkg/controller"
	extensionsController "github.com/crossplaneio/crossplane/pkg/controller/extensions"
	"github.com/crossplaneio/crossplane/pkg/extensions"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

func main() {
	var (
		log = logging.Logger

		// top level app definition
		app        = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug      = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod = app.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").
				Short('s').Default("1h").Duration()

		// default crossplane command and args, this is the default main entry point for Crossplane's
		// multi-cloud control plane functionality
		crossplaneCmd = app.Command(filepath.Base(os.Args[0]), "An open source multicloud control plane.").Default()

		// extensions commands and args, these are the main entry points for Crossplane's extension manager (EM).
		// The EM runs as a separate pod from the main Crossplane pod because in order to install extensions that
		// have arbitrary permissions, the EM itself must have cluster-admin permissions.  We isolate these elevated
		// permissions as much as possible by running the Crossplane extension manager in its own isolate deployment.
		extCmd = app.Command("extension", "Perform operations on extensions")

		// extension manage - adds the extension manager controllers and starts their reconcile loops
		extManageCmd = extCmd.Command("manage", "Manage extensions (run extension manager controllers)")

		// extension unpack - performs the unpacking operation for the given extension package content
		// directory. This command is expected to parse the content and generate manifests for extension
		// related artifacts to stdout so that the EM can read the output and use the Kubernetes API to
		// create the artifacts.
		//
		// Users are not expected to run this command themselves, the extension manager itself should
		// execute this command.
		extUnpackCmd = extCmd.Command("unpack", "Unpack an extension")
		extUnpackDir = extUnpackCmd.Flag("content-dir", "The directory that contains the extension contents").Required().String()
	)
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := runtimelog.ZapLogger(*debug)
	logging.SetLogger(zl)
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		runtimelog.SetLogger(zl)
	}

	var addToManagerFunc func(manager.Manager) error

	// Determine the command being called and execute the corresponding logic
	switch cmd {
	case crossplaneCmd.FullCommand():
		// the default Crossplane command is being run, add all the regular controllers to the manager
		addToManagerFunc = controller.AddToManager
	case extManageCmd.FullCommand():
		// the "extensions manage" command is being run, the only controllers we should add to the
		// manager are the extensions controllers
		addToManagerFunc = extensionsController.AddToManager
	case extUnpackCmd.FullCommand():
		// extension unpack command was called, run the extension unpacking logic
		kingpin.FatalIfError(extensions.Unpack(*extUnpackDir), "failed to unpack extensions")
		return
	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		kingpin.FatalIfError(err, "Cannot get config")
	}

	log.Info("Sync period", "duration", syncPeriod.String())

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{SyncPeriod: syncPeriod})
	if err != nil {
		kingpin.FatalIfError(err, "Cannot create manager")
	}

	log.Info("Adding schemes")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		kingpin.FatalIfError(err, "Cannot add APIs to scheme")
	}

	log.Info("Adding controllers")

	// Setup all Controllers
	if err := addToManagerFunc(mgr); err != nil {
		kingpin.FatalIfError(err, "Cannot add controllers to manager")
	}

	log.Info("Starting the manager")

	// Start the Cmd
	kingpin.FatalIfError(mgr.Start(signals.SetupSignalHandler()), "Cannot start controller")
}

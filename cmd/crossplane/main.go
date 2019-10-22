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
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis"
	"github.com/crossplaneio/crossplane/pkg/controller/defaultclass"
	stacksController "github.com/crossplaneio/crossplane/pkg/controller/stacks"
	templatestacksController "github.com/crossplaneio/crossplane/pkg/controller/templatestack"
	"github.com/crossplaneio/crossplane/pkg/controller/workload"
	"github.com/crossplaneio/crossplane/pkg/stacks"
	"github.com/crossplaneio/crossplane/pkg/stacks/walker"
)

// nolint: cyclomatic
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

		// stacks  commands and args, these are the main entry points for Crossplane's stack manager (SM).
		// The SM runs as a separate pod from the main Crossplane pod because in order to install stacks that
		// have arbitrary permissions, the SM itself must have cluster-admin permissions.  We isolate these elevated
		// permissions as much as possible by running the Crossplane stack manager in its own isolate deployment.
		extCmd = app.Command("stack", "Perform operations on stacks")

		// stack manage - adds the stack manager controllers and starts their reconcile loops
		extManageCmd = extCmd.Command("manage", "Manage stacks (run stack manager controllers)")

		// stack tsm - adds the template stack manager controllers and starts
		// their reconcile loops to manage a template stack
		extTSMCmd       = extCmd.Command("tsm", "Template Stack Manager (run template stack manager controllers)")
		extTSMStack     = extTSMCmd.Flag("stack", "The name of the template stack to manage").Required().String()
		extTSMNamespace = extTSMCmd.Flag("namespace", "The namespace of the template stack to manage (defaults to POD_NAMESPACE)").Default(os.Getenv("POD_NAMESPACE")).String()

		// stack unpack - performs the unpacking operation for the given stack package content
		// directory. This command is expected to parse the content and generate manifests for stack
		// related artifacts to stdout so that the SM can read the output and use the Kubernetes API to
		// create the artifacts.
		//
		// Users are not expected to run this command themselves, the stack manager itself should
		// execute this command.
		extUnpackCmd             = extCmd.Command("unpack", "Unpack a stack")
		extUnpackDir             = extUnpackCmd.Flag("content-dir", "The absolute path of the directory that contains the stack contents").Required().String()
		extUnpackOutfile         = extUnpackCmd.Flag("outfile", "The file where the YAML Stack record and CRD artifacts will be written").String()
		extUnpackPermissionScope = extUnpackCmd.Flag("permission-scope", "The permission-scope that the stack must request (Namespaced, Cluster)").Default("Namespaced").String()
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

	var setupWithManagerFunc func(manager.Manager) error

	// Determine the command being called and execute the corresponding logic
	switch cmd {
	case crossplaneCmd.FullCommand():
		// the default Crossplane command is being run, add all the regular controllers to the manager
		setupWithManagerFunc = controllerSetupWithManager
	case extManageCmd.FullCommand():
		// the "stacks manage" command is being run, the only controllers we should add to the
		// manager are the stacks controllers
		setupWithManagerFunc = stacksControllerSetupWithManager
	case extTSMCmd.FullCommand():
		// the "template stacks manager" command is being run, the only controllers we should add to the
		// manager are the template stacks controllers
		stack := types.NamespacedName{Namespace: *extTSMNamespace, Name: *extTSMStack}
		setupWithManagerFunc = tsmControllerWithStack(stack)
	case extUnpackCmd.FullCommand():
		var outFile io.StringWriter
		// stack unpack command was called, run the stack unpacking logic
		if extUnpackOutfile == nil || *extUnpackOutfile == "" {
			outFile = os.Stdout
		} else {
			openFile, err := os.Create(*extUnpackOutfile)
			kingpin.FatalIfError(err, "Cannot create outfile")
			defer closeOrError(openFile)
			outFile = openFile
		}

		// TODO(displague) afero.NewBasePathFs could avoid the need to track Base
		fs := afero.NewOsFs()
		rd := &walker.ResourceDir{Base: filepath.Clean(*extUnpackDir), Walker: afero.Afero{Fs: fs}}
		kingpin.FatalIfError(stacks.Unpack(rd, outFile, rd.Base, *extUnpackPermissionScope), "failed to unpack stacks")
		return
	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	kingpin.FatalIfError(err, "Cannot get config")

	log.Info("Sync period", "duration", syncPeriod.String())

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{SyncPeriod: syncPeriod})
	kingpin.FatalIfError(err, "Cannot create manager")

	log.Info("Adding schemes")

	// add all resources to the manager's runtime scheme
	if err := addToScheme(mgr.GetScheme()); err != nil {
		kingpin.FatalIfError(err, "Cannot add APIs to scheme")
	}

	log.Info("Adding controllers")

	// Setup all Controllers
	if err := setupWithManagerFunc(mgr); err != nil {
		kingpin.FatalIfError(err, "Cannot add controllers to manager")
	}

	log.Info("Starting the manager")

	// Start the Cmd
	kingpin.FatalIfError(mgr.Start(signals.SetupSignalHandler()), "Cannot start controller")
}

func closeOrError(c io.Closer) {
	err := c.Close()
	kingpin.FatalIfError(err, "Cannot close file")
}

func controllerSetupWithManager(mgr manager.Manager) error {
	if err := (&defaultclass.Controllers{}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&workload.Controllers{}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}

func stacksControllerSetupWithManager(mgr manager.Manager) error {
	if err := (&stacksController.Controllers{}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}

func tsmControllerWithStack(stack types.NamespacedName) func(manager.Manager) error {
	// templatestacksControllerSetupWithManager
	return func(mgr manager.Manager) error {
		controller := &templatestacksController.Controllers{Stack: stack}
		if err := controller.SetupWithManager(mgr); err != nil {
			return err
		}

		return nil
	}
}

// addToScheme adds all resources to the runtime scheme.
func addToScheme(scheme *runtime.Scheme) error {
	if err := apis.AddToScheme(scheme); err != nil {
		return err
	}

	return nil
}

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

	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/pkg/controller/stacks"
	"github.com/crossplane/crossplane/pkg/controller/stacks/templates"
	"github.com/crossplane/crossplane/pkg/controller/workload"
	stack "github.com/crossplane/crossplane/pkg/stacks"
	"github.com/crossplane/crossplane/pkg/stacks/walker"
)

func main() {
	var (
		app        = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug      = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod = app.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").Duration()

		crossplaneCmd = app.Command(filepath.Base(os.Args[0]), "Start core Crossplane controllers.").Default()

		extCmd = app.Command("stack", "Perform operations on stacks")

		// The stack manager runs as a separate pod from the core Crossplane
		// controllers because in order to install stacks that have arbitrary
		// permissions, the SM itself must have cluster-admin permissions. We
		// isolate these elevated permissions as much as possible by running the
		// Crossplane stack manager in its own isolated deployment.
		extManageCmd                 = extCmd.Command("manage", "Start Crosplane Stack Manager controllers")
		extManageTemplates           = extManageCmd.Flag("templates", "Enable support for template stacks").Bool()
		extManageTemplatesController = extManageCmd.Flag("templating-controller-image", "The image of the Template Stacks controller (implies --templates)").Default("").String()

		extManageHostControllerNamespace = extManageCmd.Flag("host-controller-namespace", "The namespace on Host Cluster where install and controller jobs/deployments will be created. Setting this will activate host aware mode of Stack Manager").String()
		extManageTenantKubeconfig        = extManageCmd.Flag("tenant-kubeconfig", "The absolute path of the kubeconfig file to Tenant Kubernetes instance (required for host aware mode, ignored otherwise).").ExistingFile()

		// Unpack the given stack package content. This command is expected to
		// parse the content and generate manifests for stack related artifacts
		// to stdout so that the SM can read the output and use the Kubernetes
		// API to create the artifacts.
		//
		// Users are not expected to run this command themselves, the stack
		// manager itself should execute this command.
		//
		// Unpack does not interact with the Kubernetes API.
		extUnpackCmd                 = extCmd.Command("unpack", "Unpack a Stack").Alias("unstack")
		extUnpackDir                 = extUnpackCmd.Flag("content-dir", "The absolute path of the directory that contains the stack contents").Required().String()
		extUnpackOutfile             = extUnpackCmd.Flag("outfile", "The file where the YAML Stack record and CRD artifacts will be written").String()
		extUnpackPermissionScope     = extUnpackCmd.Flag("permission-scope", "The permission-scope that the stack must request (Namespaced, Cluster)").Default("Namespaced").String()
		extUnpackTemplatesController = extUnpackCmd.Flag("templating-controller-image", "The image of the Template Stacks controller").Default("").String()
	)
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	// TODO(negz): Is there a reason these distinct pieces of functionality
	// should all be part of the same binary? Should we break them up?
	switch cmd {

	case crossplaneCmd.FullCommand():
		log := logging.NewLogrLogger(zl.WithName("crossplane"))
		log.Debug("Starting", "sync-period", syncPeriod.String())

		cfg, err := ctrl.GetConfig()
		kingpin.FatalIfError(err, "Cannot get config")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: syncPeriod})
		kingpin.FatalIfError(err, "Cannot create manager")

		kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add core Crossplane APIs to scheme")
		kingpin.FatalIfError(workload.Setup(mgr, log), "Cannot setup workload controllers")
		kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")

	case extManageCmd.FullCommand():
		log := logging.NewLogrLogger(zl.WithName("stack-manager"))
		log.Debug("Starting", "sync-period", syncPeriod.String())

		cfg, err := getRestConfig(*extManageTenantKubeconfig)
		kingpin.FatalIfError(err, "Cannot get config")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: syncPeriod})
		kingpin.FatalIfError(err, "Cannot create manager")

		kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add core Crossplane APIs to scheme")
		kingpin.FatalIfError(apiextensionsv1beta1.AddToScheme(mgr.GetScheme()), "Cannot add API extensions to scheme")
		kingpin.FatalIfError(stacks.Setup(mgr, log, *extManageHostControllerNamespace, *extManageTemplatesController), "Cannot add stacks controllers to manager")

		if *extManageTemplatesController != "" {
			*extManageTemplates = true
		}

		if *extManageTemplates {
			if *extManageTemplatesController == "" {
				kingpin.Fatalf("--templating-controller-image is required with --templates")
			}

			kingpin.FatalIfError(templates.SetupStackDefinitions(mgr, log), "Cannot add stack definition controller to manager")
		}

		kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")

	case extUnpackCmd.FullCommand():
		log := logging.NewLogrLogger(zl.WithName("stacks"))

		outFile := os.Stdout
		if *extUnpackOutfile != "" {
			f, err := os.Create(*extUnpackOutfile)
			kingpin.FatalIfError(err, "Cannot create output file")
			defer kingpin.FatalIfError(f.Close(), "Cannot close file")
			outFile = f
		}
		log.Debug("Unpacking stack", "to", outFile.Name())

		// TODO(displague) afero.NewBasePathFs could avoid the need to track Base
		fs := afero.NewOsFs()
		rd := &walker.ResourceDir{Base: filepath.Clean(*extUnpackDir), Walker: afero.Afero{Fs: fs}}
		kingpin.FatalIfError(stack.Unpack(rd, outFile, rd.Base, *extUnpackPermissionScope, *extUnpackTemplatesController), "failed to unpack stacks")

	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}
}

func getRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		return ctrl.GetConfig()
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
}

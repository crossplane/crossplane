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
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/pkg/controller/oam"
	"github.com/crossplane/crossplane/pkg/controller/stacks"
	"github.com/crossplane/crossplane/pkg/controller/stacks/templates"
	"github.com/crossplane/crossplane/pkg/controller/workload"
	stack "github.com/crossplane/crossplane/pkg/stacks"
	"github.com/crossplane/crossplane/pkg/stacks/walker"
)

type fullCommand struct {
	clause  *kingpin.CmdClause
	handler func()
}

type appReq struct {
	app        *kingpin.Application
	zl         logr.Logger
	syncPeriod *time.Duration
}

func main() {
	baseCmd := filepath.Base(os.Args[0])
	args := os.Args[1:]

	app := kingpin.New(baseCmd, "An open source multicloud control plane.").DefaultEnvars()
	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	syncPeriod := app.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").Duration()

	// populate debug early
	_, _ = app.Parse(args)

	a := appReq{
		app:        app,
		zl:         zap.New(zap.UseDevMode(*debug)),
		syncPeriod: syncPeriod,
	}

	commands := []fullCommand{}
	commands = append(commands, a.crossplaneCmd(baseCmd)...)
	commands = append(commands, a.stackCmd()...)

	cmdGiven := kingpin.MustParse(app.Parse(args))

	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(a.zl)
	}

	knownCmd := false
	for _, cmd := range commands {
		if cmdGiven == cmd.clause.FullCommand() {
			knownCmd = true
			cmd.handler()
			break
		}
	}

	if !knownCmd {
		kingpin.FatalUsage("unknown command %s", cmdGiven)
	}
}

// crossplaneCmd provides the fullCommand for the default command
//
// The default command runs the controllers for all Crossplane resources except
// stack resources.
func (a appReq) crossplaneCmd(baseCmd string) []fullCommand {
	crossplaneCmd := a.app.Command(baseCmd, "Start core Crossplane controllers.").Default()

	return []fullCommand{{crossplaneCmd, func() {
		log := logging.NewLogrLogger(a.zl.WithName("crossplane"))
		log.Debug("Starting", "sync-period", a.syncPeriod.String())

		cfg, err := ctrl.GetConfig()
		kingpin.FatalIfError(err, "Cannot get config")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: a.syncPeriod})
		kingpin.FatalIfError(err, "Cannot create manager")

		kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add core Crossplane APIs to scheme")
		kingpin.FatalIfError(oam.Setup(mgr, log), "Cannot setup OAM controllers")
		kingpin.FatalIfError(workload.Setup(mgr, log), "Cannot setup workload controllers")
		kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
	}}}
}

// stackCmd provides the fullCommand for the "stack" command
func (a appReq) stackCmd() []fullCommand {
	stackCmd := a.app.Command("stack", "Perform operations on stacks")
	return []fullCommand{
		a.stackManageCmd(stackCmd),
		a.stackUnpackCmd(stackCmd),
	}
}

// stackManageCmd provides the fullCommand for the "stack manage" command
//
// The stack manager runs as a separate pod from the core Crossplane
// controllers because in order to install stacks that have arbitrary
// permissions, the SM itself must have cluster-admin permissions. We
// isolate these elevated permissions as much as possible by running the
// Crossplane stack manager in its own isolated deployment.
func (a appReq) stackManageCmd(stackCmd *kingpin.CmdClause) fullCommand {
	stackManageCmd := stackCmd.Command("manage", "Start Crossplane Stack Manager controllers")

	stackManageRestrictCore := stackManageCmd.Flag("restrict-core-apigroups", "Enable API group restrictions for Stacks. When enabled, APIs that Stacks depend on and own must contain a dot (\".\") and may not end with \"k8s.io\". When omitted, all groups are permitted.").Default("false").Bool()
	stackManageTemplates := stackManageCmd.Flag("templates", "Enable support for template stacks").Bool()
	stackManageTemplatesController := stackManageCmd.Flag("templating-controller-image", "The image of the Template Stacks controller (implies --templates)").Default("").String()

	stackManageHostControllerNamespace := stackManageCmd.Flag("host-controller-namespace", "The namespace on Host Cluster where install and controller jobs/deployments will be created. Setting this will activate host aware mode of Stack Manager").String()
	stackManageTenantKubeconfig := stackManageCmd.Flag("tenant-kubeconfig", "The absolute path of the kubeconfig file to Tenant Kubernetes instance (required for host aware mode, ignored otherwise).").ExistingFile()

	return fullCommand{stackManageCmd, func() {
		log := logging.NewLogrLogger(a.zl.WithName(stack.LabelValueStackManager))
		log.Debug("Starting", "sync-period", a.syncPeriod.String())

		if *stackManageRestrictCore {
			log.Debug("Restricting core group use in the Stacks")
		}

		cfg, err := getRestConfig(*stackManageTenantKubeconfig)
		kingpin.FatalIfError(err, "Cannot get config")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: a.syncPeriod})
		kingpin.FatalIfError(err, "Cannot create manager")

		kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add core Crossplane APIs to scheme")
		kingpin.FatalIfError(apiextensionsv1beta1.AddToScheme(mgr.GetScheme()), "Cannot add API extensions to scheme")
		kingpin.FatalIfError(stacks.Setup(mgr, log, *stackManageHostControllerNamespace, *stackManageTemplatesController, *stackManageRestrictCore), "Cannot add stacks controllers to manager")

		if *stackManageTemplatesController != "" {
			*stackManageTemplates = true
		}

		if *stackManageTemplates {
			if *stackManageTemplatesController == "" {
				kingpin.Fatalf("--templating-controller-image is required with --templates")
			}

			kingpin.FatalIfError(templates.SetupStackDefinitions(mgr, log), "Cannot add stack definition controller to manager")
		}

		kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
	}}
}

// stackUnpackCmd provides the fullCommand for the "stack unpack" command
//
// Unpack the given stack package content. This command is expected to
// parse the content and generate manifests for stack related artifacts
// to stdout so that the SM can read the output and use the Kubernetes
// API to create the artifacts.
//
// Users are not expected to run this command themselves, the stack
// manager itself should execute this command.
//
// Unpack does not interact with the Kubernetes API.
func (a appReq) stackUnpackCmd(stackCmd *kingpin.CmdClause) fullCommand {
	stackUnpackCmd := stackCmd.Command("unpack", "Unpack a Stack").Alias("unstack")
	stackUnpackDir := stackUnpackCmd.Flag("content-dir", "The absolute path of the directory that contains the stack contents").Required().String()
	stackUnpackOutfile := stackUnpackCmd.Flag("outfile", "The file where the YAML Stack record and CRD artifacts will be written").String()
	stackUnpackPermissionScope := stackUnpackCmd.Flag("permission-scope", "The permission-scope that the stack must request (Namespaced, Cluster)").Default("Namespaced").String()
	stackUnpackTemplatesController := stackUnpackCmd.Flag("templating-controller-image", "The image of the Template Stacks controller").Default("").String()

	return fullCommand{stackUnpackCmd, func() {
		log := logging.NewLogrLogger(a.zl.WithName("stacks"))

		outFile := os.Stdout
		if *stackUnpackOutfile != "" {
			f, err := os.Create(*stackUnpackOutfile)
			kingpin.FatalIfError(err, "Cannot create output file")
			defer kingpin.FatalIfError(f.Close(), "Cannot close file")
			outFile = f
		}
		log.Debug("Unpacking stack", "to", outFile.Name())

		// TODO(displague) afero.NewBasePathFs could avoid the need to track Base
		fs := afero.NewOsFs()
		rd := &walker.ResourceDir{Base: filepath.Clean(*stackUnpackDir), Walker: afero.Afero{Fs: fs}}
		kingpin.FatalIfError(stack.Unpack(rd, outFile, rd.Base, *stackUnpackPermissionScope, *stackUnpackTemplatesController, log), "failed to unpack stacks")
	}}
}

func getRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		return ctrl.GetConfig()
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
}

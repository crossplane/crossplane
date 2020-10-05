/*
Copyright 2020 The Crossplane Authors.

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

package configuration

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/cmd/crank/pkg"
	typedclient "github.com/crossplane/crossplane/pkg/client/clientset/versioned/typed/pkg/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/pkg/revision"
)

// Cmd is the root command for Configurations.
type Cmd struct {
	Build   BuildCmd    `cmd:"" help:"Build a Configuration."`
	Lint    LintCmd     `cmd:"" help:"Lint the contents of a Configuration Package."`
	Push    pkg.PushCmd `cmd:"" help:"Push a Configuration Package."`
	Install InstallCmd  `cmd:"" help:"Install a Configuration."`
}

// Run runs the Configuration command.
func (r *Cmd) Run() error {
	return nil
}

// BuildCmd builds a Configuration.
type BuildCmd struct {
	pkg.BuildCmd
}

// Run runs the configuration build cmd.
func (pbc *BuildCmd) Run() error {
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	err = pbc.BuildCmd.Build(ctx, docker)
	if err != nil {
		return err
	}

	if pbc.NoLint {
		return nil
	}
	lc := LintCmd{
		pkg.LintCmd{
			Image:  pbc.FullImageName(),
			NoPull: pbc.NoPull,
		},
		ctx,
		docker,
	}
	return lc.Run()
}

// LintCmd lints the contents of a Configuration Package.
type LintCmd struct {
	pkg.LintCmd
	ctx    context.Context
	docker *client.Client
}

// Run runs the configuration lint cmd
func (pc *LintCmd) Run() error {
	ctx := context.Background()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	linter := revision.NewPackageLinter(
		revision.PackageLinterFns(revision.OneMeta),
		revision.ObjectLinterFns(revision.IsConfiguration),
		revision.ObjectLinterFns(revision.IsXRD))
	return pc.LintCmd.Lint(ctx, docker, linter)
}

// InstallCmd creates a Configuration in the cluster.
type InstallCmd struct {
	Package string `arg:"" name:"package" help:"Image containing Configuration package."`

	Name                 string `optional:"" name:"name" help:"Name of Configuration."`
	RevisionHistoryLimit int64  `short:"rl" help:"Revision history limit."`
	ManualActivation     bool   `short:"m" help:"Enable manual revision activation policy."`
}

// Run the InstallCmd.
func (p *InstallCmd) Run() error {
	ctx := context.TODO()
	rap := v1alpha1.AutomaticActivation
	if p.ManualActivation {
		rap = v1alpha1.ManualActivation
	}
	name := p.Name
	if name == "" {
		// NOTE(muvaf): "crossplane/provider-gcp:master" -> "provider-gcp"
		woTag := strings.Split(strings.Split(p.Package, ":")[0], "/")
		name = woTag[len(woTag)-1]
	}
	cr := &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ConfigurationSpec{
			PackageSpec: v1alpha1.PackageSpec{
				Package:                  p.Package,
				RevisionActivationPolicy: &rap,
				RevisionHistoryLimit:     &p.RevisionHistoryLimit,
			},
		},
	}
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	res, err := kube.Configurations().Create(ctx, cr, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot create configuration")
	}
	fmt.Printf("%s/%s is created\n", strings.ToLower(v1alpha1.ConfigurationGroupKind), res.GetName())
	return nil
}

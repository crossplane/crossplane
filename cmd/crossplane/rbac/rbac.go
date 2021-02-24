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

package rbac

import (
	"context"
	"fmt"

	"time"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/rbac"
	"github.com/crossplane/crossplane/internal/initializer"
)

// Available RBAC management policies.
const (
	ManagementPolicyAll   = string(rbac.ManagementPolicyAll)
	ManagementPolicyBasic = string(rbac.ManagementPolicyBasic)
)

// Command configuration for the RBAC manager.
type Command struct {
	Name                string
	Sync                time.Duration
	LeaderElection      bool
	ManagementPolicy    string
	ProviderClusterRole string
}

// FromKingpin produces the RBAC manager command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("manage", "RBAC management policy.").Short('m').Default(ManagementPolicyAll).EnumVar(&c.ManagementPolicy, ManagementPolicyAll, ManagementPolicyBasic)
	cmd.Flag("provider-clusterrole", "A ClusterRole enumerating the permissions provider packages may request.").StringVar(&c.ProviderClusterRole)
	cmd.Flag("leader-election", "Use leader election for the conroller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").BoolVar(&c.LeaderElection)

	return c
}

// Run the RBAC manager.
func (c *Command) Run(log logging.Logger) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	s := runtime.NewScheme()
	for _, f := range []func(scheme *runtime.Scheme) error{
		scheme.AddToScheme,
		extv1.AddToScheme,
		apis.AddToScheme,
	} {
		if err := f(s); err != nil {
			return err
		}
	}
	cl, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, "cannot create new kubernetes client")
	}
	// NOTE(muvaf): The plural form of the kind name is not available in Go code.
	i := initializer.New(cl,
		initializer.NewCRDWaiter([]string{
			fmt.Sprintf("%s.%s", "compositeresourcedefinitions", v1.Group),
			fmt.Sprintf("%s.%s", "providerrevisions", pkgv1.Group),
		}, time.Minute, log),
	)
	if err := i.Init(context.TODO()); err != nil {
		return errors.Wrap(err, "cannot initialize rbac manager")
	}
	log.Info("Initialization has been completed")

	log.Debug("Starting", "sync-period", c.Sync.String(), "policy", c.ManagementPolicy)
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           s,
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: fmt.Sprintf("crossplane-leader-election-%s", c.Name),
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create manager")
	}

	if err := rbac.Setup(mgr, log, rbac.ManagementPolicy(c.ManagementPolicy), c.ProviderClusterRole); err != nil {
		return errors.Wrap(err, "cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}

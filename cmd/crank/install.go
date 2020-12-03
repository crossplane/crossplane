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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	typedclient "github.com/crossplane/crossplane/internal/client/clientset/versioned/typed/pkg/v1beta1"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// installCmd installs a package.
type installCmd struct {
	Configuration installConfigCmd   `cmd:"" help:"Install a Configuration package."`
	Provider      installProviderCmd `cmd:"" help:"Install a Provider package."`
}

// Run runs the install cmd.
func (c *installCmd) Run(b *buildChild) error {
	return nil
}

// installConfigCmd installs a Configuration.
type installConfigCmd struct {
	Package string `arg:"" help:"Image containing Configuration package."`

	Name                 string   `arg:"" optional:"" help:"Name of Configuration."`
	RevisionHistoryLimit int64    `short:"rl" help:"Revision history limit."`
	ManualActivation     bool     `short:"m" help:"Enable manual revision activation policy."`
	PackagePullSecrets   []string `help:"List of secrets used to pull package."`
}

// Run runs the Configuration install cmd.
func (c *installConfigCmd) Run(k *kong.Context) error {
	rap := v1beta1.AutomaticActivation
	if c.ManualActivation {
		rap = v1beta1.ManualActivation
	}
	name := c.Name
	if name == "" {
		// NOTE(muvaf): "crossplane/my-configuration:master" -> "my-configuration"
		woTag := strings.Split(strings.Split(c.Package, ":")[0], "/")
		name = woTag[len(woTag)-1]
	}
	packagePullSecrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		packagePullSecrets[i] = corev1.LocalObjectReference{
			Name: s,
		}
	}
	cr := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.ConfigurationSpec{
			PackageSpec: v1beta1.PackageSpec{
				Package:                  c.Package,
				RevisionActivationPolicy: &rap,
				RevisionHistoryLimit:     &c.RevisionHistoryLimit,
				PackagePullSecrets:       packagePullSecrets,
			},
		},
	}
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	res, err := kube.Configurations().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot create configuration")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1beta1.ConfigurationGroupKind), res.GetName())
	return err
}

// installProviderCmd install a Provider.
type installProviderCmd struct {
	Package string `arg:"" help:"Image containing Provider package."`

	Name                 string `arg:"" optional:"" help:"Name of Provider."`
	RevisionHistoryLimit int64  `short:"rl" help:"Revision history limit."`
	ManualActivation     bool   `short:"m" help:"Enable manual revision activation policy."`
}

// Run runs the Provider install cmd.
func (c *installProviderCmd) Run(k *kong.Context) error {
	rap := v1beta1.AutomaticActivation
	if c.ManualActivation {
		rap = v1beta1.ManualActivation
	}
	name := c.Name
	if name == "" {
		// NOTE(muvaf): "crossplane/provider-gcp:master" -> "provider-gcp"
		woTag := strings.Split(strings.Split(c.Package, ":")[0], "/")
		name = woTag[len(woTag)-1]
	}
	cr := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.ProviderSpec{
			PackageSpec: v1beta1.PackageSpec{
				Package:                  c.Package,
				RevisionActivationPolicy: &rap,
				RevisionHistoryLimit:     &c.RevisionHistoryLimit,
			},
		},
	}
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	res, err := kube.Providers().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot create provider")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1beta1.ProviderGroupKind), res.GetName())
	return err
}

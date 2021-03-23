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
	"net/http"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/xpkg"
	typedclient "github.com/crossplane/crossplane/pkg/client/clientset/versioned/typed/pkg/v1"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	errPkgIdentifier = "invalid package image identifier"
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
	RevisionHistoryLimit int64    `short:"r" help:"Revision history limit."`
	ManualActivation     bool     `short:"m" help:"Enable manual revision activation policy."`
	PackagePullSecrets   []string `help:"List of secrets used to pull package."`
}

// Run runs the Configuration install cmd.
func (c *installConfigCmd) Run(k *kong.Context) error {
	rap := v1.AutomaticActivation
	if c.ManualActivation {
		rap = v1.ManualActivation
	}
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package)
		if err != nil {
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}
	packagePullSecrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		packagePullSecrets[i] = corev1.LocalObjectReference{
			Name: s,
		}
	}
	cr := &v1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgName,
		},
		Spec: v1.ConfigurationSpec{
			PackageSpec: v1.PackageSpec{
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
		return errors.Wrap(warnIfNotFound(err), "cannot create configuration")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1.ConfigurationGroupKind), res.GetName())
	return err
}

// installProviderCmd install a Provider.
type installProviderCmd struct {
	Package string `arg:"" help:"Image containing Provider package."`

	Name                 string   `arg:"" optional:"" help:"Name of Provider."`
	RevisionHistoryLimit int64    `short:"r" help:"Revision history limit."`
	ManualActivation     bool     `short:"m" help:"Enable manual revision activation policy."`
	PackagePullSecrets   []string `help:"List of secrets used to pull package."`
}

// Run runs the Provider install cmd.
func (c *installProviderCmd) Run(k *kong.Context) error {
	rap := v1.AutomaticActivation
	if c.ManualActivation {
		rap = v1.ManualActivation
	}
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package)
		if err != nil {
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}
	packagePullSecrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		packagePullSecrets[i] = corev1.LocalObjectReference{
			Name: s,
		}
	}
	cr := &v1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgName,
		},
		Spec: v1.ProviderSpec{
			PackageSpec: v1.PackageSpec{
				Package:                  c.Package,
				RevisionActivationPolicy: &rap,
				RevisionHistoryLimit:     &c.RevisionHistoryLimit,
				PackagePullSecrets:       packagePullSecrets,
			},
		},
	}
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	res, err := kube.Providers().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot create provider")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1.ProviderGroupKind), res.GetName())
	return err
}

func warnIfNotFound(err error) error {
	serr, ok := err.(*apierrors.StatusError)
	if !ok {
		return err
	}
	if serr.ErrStatus.Code != http.StatusNotFound {
		return err
	}
	return errors.WithMessagef(err, "kubectl-crossplane plugin %s might be out of date", version.New().GetVersionString())
}

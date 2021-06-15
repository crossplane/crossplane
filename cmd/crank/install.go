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
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	typedclient "github.com/crossplane/crossplane/internal/client/clientset/versioned/typed/pkg/v1"
	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/xpkg"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	errPkgIdentifier = "invalid package image identifier"
	errKubeConfig    = "failed to get kubeconfig"
	errKubeClient    = "failed to create kube client"

	errFmtPkgNotReadyTimeout = "%s is not ready in timeout duration"
	errFmtWatchPkg           = "Failed to watch for %s object"
)

const (
	msgConfigurationReady    = "Configuration is ready"
	msgConfigurationNotReady = "Configuration is not ready"
	msgConfigurationWaiting  = "Waiting for the Configuration to be ready"

	msgProviderReady    = "Provider is ready"
	msgProviderNotReady = "Provider is not ready"
	msgProviderWaiting  = "Waiting for the Provider to be ready"
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

	Name                 string        `arg:"" optional:"" help:"Name of Configuration."`
	Wait                 time.Duration `short:"w" help:"Wait for installation of package."`
	RevisionHistoryLimit int64         `short:"r" help:"Revision history limit."`
	ManualActivation     bool          `short:"m" help:"Enable manual revision activation policy."`
	PackagePullSecrets   []string      `help:"List of secrets used to pull package."`
}

// Run runs the Configuration install cmd.
func (c *installConfigCmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo
	rap := v1.AutomaticActivation
	if c.ManualActivation {
		rap = v1.ManualActivation
	}
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package)
		if err != nil {
			logger.Debug(errPkgIdentifier, "error", err)
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}
	logger = logger.WithValues("configurationName", pkgName)
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
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")
	kube, err := typedclient.NewForConfig(kubeConfig)
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeClient)
	}
	logger.Debug("Created Kubernetes client")
	res, err := kube.Configurations().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		logger.Debug("Failed to create configuration", "error", warnIfNotFound(err))
		return errors.Wrap(warnIfNotFound(err), "cannot create configuration")
	}
	if c.Wait != 0 {
		logger.Debug(msgConfigurationWaiting)
		watchList := cache.NewListWatchFromClient(kube.RESTClient(), "configurations", corev1.NamespaceAll, fields.Everything())
		waitSeconds := int64(c.Wait.Seconds())
		watcher, err := watchList.Watch(metav1.ListOptions{Watch: true, TimeoutSeconds: &waitSeconds})
		defer watcher.Stop()
		if err != nil {
			logger.Debug(fmt.Sprintf(errFmtWatchPkg, "Configuration"), "error", err)
			return err
		}
		for {
			event, ok := <-watcher.ResultChan()
			if !ok {
				logger.Debug(fmt.Sprintf(errFmtPkgNotReadyTimeout, "Configuration"))
				return errors.Errorf(errFmtPkgNotReadyTimeout, "Configuration")
			}
			obj := (event.Object).(*v1.Configuration)
			cond := obj.GetCondition(v1.TypeHealthy)
			if obj.ObjectMeta.Name == pkgName && cond.Status == corev1.ConditionTrue {
				logger.Debug(msgConfigurationReady)
				break
			}
			logger.Debug(msgConfigurationNotReady)
		}
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1.ConfigurationGroupKind), res.GetName())
	return err
}

// installProviderCmd install a Provider.
type installProviderCmd struct {
	Package string `arg:"" help:"Image containing Provider package."`

	Name                 string        `arg:"" optional:"" help:"Name of Provider."`
	Wait                 time.Duration `short:"w" help:"Wait for installation of package"`
	RevisionHistoryLimit int64         `short:"r" help:"Revision history limit."`
	ManualActivation     bool          `short:"m" help:"Enable manual revision activation policy."`
	Config               string        `help:"Specify a ControllerConfig for this Provider."`
	PackagePullSecrets   []string      `help:"List of secrets used to pull package."`
}

// Run runs the Provider install cmd.
func (c *installProviderCmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo
	rap := v1.AutomaticActivation
	if c.ManualActivation {
		rap = v1.ManualActivation
	}
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package)
		if err != nil {
			logger.Debug(errPkgIdentifier, "error", err)
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}
	logger = logger.WithValues("providerName", pkgName)
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
	if c.Config != "" {
		cr.Spec.ControllerConfigReference = &xpv1.Reference{
			Name: c.Config,
		}
	}
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")
	kube, err := typedclient.NewForConfig(kubeConfig)
	if err != nil {
		logger.Debug(errKubeClient, "error", err)
		return errors.Wrap(err, errKubeClient)
	}
	logger.Debug("Created kubernetes client")
	res, err := kube.Providers().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		logger.Debug("Failed to create provider", "error", warnIfNotFound(err))
		return errors.Wrap(warnIfNotFound(err), "cannot create provider")
	}
	if c.Wait != 0 {
		logger.Debug(msgProviderWaiting)
		watchList := cache.NewListWatchFromClient(kube.RESTClient(), "providers", corev1.NamespaceAll, fields.Everything())
		waitSeconds := int64(c.Wait.Seconds())
		watcher, err := watchList.Watch(metav1.ListOptions{Watch: true, TimeoutSeconds: &waitSeconds})
		defer watcher.Stop()
		if err != nil {
			logger.Debug(fmt.Sprintf(errFmtWatchPkg, "Provider"), "error", err)
			return err
		}
		for {
			event, ok := <-watcher.ResultChan()
			if !ok {
				logger.Debug(fmt.Sprintf(errFmtPkgNotReadyTimeout, "Provider"))
				return errors.Errorf(errFmtPkgNotReadyTimeout, "Provider")
			}
			obj := (event.Object).(*v1.Provider)
			cond := obj.GetCondition(v1.TypeHealthy)
			if obj.ObjectMeta.Name == pkgName && cond.Status == corev1.ConditionTrue {
				logger.Debug(msgProviderReady, "pkgName", obj.ObjectMeta.Name)
				break
			}
			logger.Debug(msgProviderNotReady)
		}
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

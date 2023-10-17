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

package xpkg

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	clientpkgv1beta1 "github.com/crossplane/crossplane/internal/client/clientset/versioned/typed/pkg/v1beta1"
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
	msgFunctionReady    = "Function is ready"
	msgFunctionNotReady = "Function is not ready"
	msgFunctionWaiting  = "Waiting for the Function to be ready"
)

// TODO(negz): These install<T>Cmd implementations are all identical. Can they
// be deduplicated into one reusable implementation?

type installCmd struct {
	Function installFunctionCmd `cmd:"" help:"Install a Function package."`
}

// installFunctionCmd installs a Function.
type installFunctionCmd struct {
	Package string `arg:"" help:"Image containing Function package."`

	Name                 string        `arg:"" optional:"" help:"Name of Function."`
	Wait                 time.Duration `short:"w" help:"Wait for installation of package"`
	RevisionHistoryLimit int64         `short:"r" help:"Revision history limit."`
	ManualActivation     bool          `short:"m" help:"Enable manual revision activation policy."`
	Config               string        `help:"Specify a DeploymentRuntimeConfig for this Function."`
	PackagePullSecrets   []string      `help:"List of secrets used to pull package."`
}

// Run runs the Function install cmd.
func (c *installFunctionCmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo // TODO(negz): Can anything be broken out here?
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
	cr := &v1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgName,
		},
		Spec: v1beta1.FunctionSpec{
			PackageSpec: v1.PackageSpec{
				Package:                  c.Package,
				RevisionActivationPolicy: &rap,
				RevisionHistoryLimit:     &c.RevisionHistoryLimit,
				PackagePullSecrets:       packagePullSecrets,
			},
		},
	}
	if c.Config != "" {
		cr.Spec.RuntimeConfigReference = &v1.RuntimeConfigReference{Name: c.Config}
	}
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")
	kube, err := clientpkgv1beta1.NewForConfig(kubeConfig)
	if err != nil {
		logger.Debug(errKubeClient, "error", err)
		return errors.Wrap(err, errKubeClient)
	}
	logger.Debug("Created kubernetes client")
	res, err := kube.Functions().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil {
		logger.Debug("Failed to create function", "error", warnIfNotFound(err))
		return errors.Wrap(warnIfNotFound(err), "cannot create function")
	}
	if c.Wait != 0 {
		logger.Debug(msgFunctionWaiting)
		watchList := cache.NewListWatchFromClient(kube.RESTClient(), "functions", corev1.NamespaceAll, fields.Everything())
		waitSeconds := int64(c.Wait.Seconds())
		watcher, err := watchList.Watch(metav1.ListOptions{Watch: true, TimeoutSeconds: &waitSeconds})
		defer watcher.Stop()
		if err != nil {
			logger.Debug(fmt.Sprintf(errFmtWatchPkg, "Function"), "error", err)
			return err
		}
		for {
			event, ok := <-watcher.ResultChan()
			if !ok {
				logger.Debug(fmt.Sprintf(errFmtPkgNotReadyTimeout, "Function"))
				return errors.Errorf(errFmtPkgNotReadyTimeout, "Function")
			}
			obj := (event.Object).(*v1beta1.Function)
			cond := obj.GetCondition(v1.TypeHealthy)
			if obj.ObjectMeta.Name == pkgName && cond.Status == corev1.ConditionTrue {
				logger.Debug(msgFunctionReady, "pkgName", obj.ObjectMeta.Name)
				break
			}
			logger.Debug(msgFunctionNotReady)
		}
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", strings.ToLower(v1beta1.FunctionGroupKind), res.GetName())
	return err
}

func warnIfNotFound(err error) error {
	serr, ok := err.(*kerrors.StatusError) //nolint:errorlint // we need to be able to extract the underlying typed error
	if !ok {
		return err
	}
	if serr.ErrStatus.Code != http.StatusNotFound {
		return err
	}
	return errors.WithMessagef(err, "CLI crossplane %s might be out of date", version.New().GetVersionString())
}

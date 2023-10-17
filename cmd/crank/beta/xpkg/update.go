/*
Copyright 2023 The Crossplane Authors.

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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	clientpkgv1beta1 "github.com/crossplane/crossplane/internal/client/clientset/versioned/typed/pkg/v1beta1"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// updateCmd updates a package.
type updateCmd struct {
	Function updateFunctionCmd `cmd:"" help:"Update a Function package."`
}

// updateFunctionCmd update a Function.
type updateFunctionCmd struct {
	Name string `arg:"" help:"Name of Function."`
	Tag  string `arg:"" help:"Updated tag for Function package."`
}

// Run runs the Function update cmd.
func (c *updateFunctionCmd) Run(k *kong.Context, logger logging.Logger) error {
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
	preProv, err := kube.Functions().Get(context.Background(), c.Name, metav1.GetOptions{})
	if err != nil {
		err = warnIfNotFound(err)
		logger.Debug("Failed to update function", "error", err)
		return errors.Wrap(err, "cannot update function")
	}
	logger.Debug("Found previous function object")
	pkg := preProv.Spec.Package
	pkgReference, err := name.ParseReference(pkg, name.WithDefaultRegistry(""))
	if err != nil {
		err = warnIfNotFound(err)
		logger.Debug("Failed to update function", "error", err)
		return errors.Wrap(err, "cannot update function")
	}
	newPkg := ""
	if strings.HasPrefix(c.Tag, "sha256") {
		newPkg = pkgReference.Context().Digest(c.Tag).Name()
	} else {
		newPkg = pkgReference.Context().Tag(c.Tag).Name()
	}
	preProv.Spec.Package = newPkg
	req, err := json.Marshal(preProv)
	if err != nil {
		err = warnIfNotFound(err)
		logger.Debug("Failed to update function", "error", err)
		return errors.Wrap(err, "cannot update function")
	}
	res, err := kube.Functions().Patch(context.Background(), c.Name, types.MergePatchType, req, metav1.PatchOptions{})
	if err != nil {
		err = warnIfNotFound(err)
		logger.Debug("Failed to update function", "error", err)
		return errors.Wrap(err, "cannot update function")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s updated\n", strings.ToLower(v1beta1.FunctionGroupKind), res.GetName())
	return err
}

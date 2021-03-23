package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	typedclient "github.com/crossplane/crossplane/pkg/client/clientset/versioned/typed/pkg/v1"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// updateCmd updates a package.
type updateCmd struct {
	Configuration updateConfigCmd   `cmd:"" help:"Update a Configuration package."`
	Provider      updateProviderCmd `cmd:"" help:"Update a Provider package."`
}

// Run runs the update cmd.
func (c *updateCmd) Run(b *buildChild) error {
	return nil
}

// updateConfigCmd updates a Configuration.
type updateConfigCmd struct {
	Name string `arg:"" help:"Name of Configuration."`
	Tag  string `arg:"" help:"Updated tag for Configuration package."`
}

// Run runs the Configuration update cmd.
func (c *updateConfigCmd) Run(k *kong.Context) error { // nolint:gocyclo
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	prevConf, err := kube.Configurations().Get(context.Background(), c.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update configuration")
	}
	pkg := prevConf.Spec.Package
	pkgReference, err := name.ParseReference(pkg)
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update configuration")
	}
	newPkg := ""
	if strings.HasPrefix(c.Tag, "sha256") {
		newPkg = pkgReference.Context().Digest(c.Tag).Name()
	} else {
		newPkg = pkgReference.Context().Tag(c.Tag).Name()
	}
	prevConf.Spec.Package = newPkg
	req, err := json.Marshal(prevConf)
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update configuration")
	}
	res, err := kube.Configurations().Patch(context.Background(), c.Name, types.MergePatchType, req, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update configuration")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s updated\n", strings.ToLower(v1.ConfigurationGroupKind), res.GetName())
	return err
}

// updateProviderCmd update a Provider.
type updateProviderCmd struct {
	Name string `arg:"" help:"Name of Provider."`
	Tag  string `arg:"" help:"Updated tag for Provider package."`
}

// Run runs the Provider update cmd.
func (c *updateProviderCmd) Run(k *kong.Context) error { // nolint:gocyclo
	kube := typedclient.NewForConfigOrDie(ctrl.GetConfigOrDie())
	preProv, err := kube.Providers().Get(context.Background(), c.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update provider")
	}
	pkg := preProv.Spec.Package
	pkgReference, err := name.ParseReference(pkg)
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update provider")
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
		return errors.Wrap(warnIfNotFound(err), "cannot update provider")
	}
	res, err := kube.Providers().Patch(context.Background(), c.Name, types.MergePatchType, req, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot update provider")
	}
	_, err = fmt.Fprintf(k.Stdout, "%s/%s updated\n", strings.ToLower(v1.ProviderGroupKind), res.GetName())
	return err
}

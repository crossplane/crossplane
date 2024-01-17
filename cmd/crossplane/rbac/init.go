// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package rbac

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/initializer"
)

// initCommand configuration for the initialization of RBAC controllers.
type initCommand struct{}

// Run starts the initialization process.
func (c *initCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	cl, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, "cannot create new kubernetes client")
	}
	// NOTE(muvaf): The plural form of the kind name is not available in Go code.
	i := initializer.New(cl, log,
		initializer.NewCRDWaiter([]string{
			fmt.Sprintf("%s.%s", "compositeresourcedefinitions", v1.Group),
			fmt.Sprintf("%s.%s", "providerrevisions", pkgv1.Group),
		}, time.Minute, time.Second, log),
	)
	if err := i.Init(context.TODO()); err != nil {
		return errors.Wrap(err, "cannot initialize core")
	}
	log.Info("Initialization has been completed")
	return nil
}

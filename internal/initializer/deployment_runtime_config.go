// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	errCreateDefaultRuntimeConfig = "cannot create DeploymentRuntimeConfig \"default\""
)

// DefaultDeploymentRuntimeConfig creates a "default" DeploymentRuntimeConfig
// object. It is a no-op if the object already exists.
func DefaultDeploymentRuntimeConfig(ctx context.Context, kube client.Client) error {
	rc := &v1beta1.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	return errors.Wrap(resource.Ignore(kerrors.IsAlreadyExists, kube.Create(ctx, rc)), errCreateDefaultRuntimeConfig)
}

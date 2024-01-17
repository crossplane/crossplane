// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	scv1alpha1 "github.com/crossplane/crossplane/apis/secrets/v1alpha1"
)

const (
	errCreateDefaultStoreConfig = "cannot create default store config"
)

// NewStoreConfigObject returns a new *StoreConfigObject initializer.
func NewStoreConfigObject(ns string) *StoreConfigObject {
	return &StoreConfigObject{
		namespace: ns,
	}
}

// StoreConfigObject has the initializer for creating the default secret
// StoreConfig.
type StoreConfigObject struct {
	namespace string
}

// Run makes sure a StoreConfig named as default exists.
func (so *StoreConfigObject) Run(ctx context.Context, kube client.Client) error {
	sc := &scv1alpha1.StoreConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Spec: scv1alpha1.StoreConfigSpec{
			// NOTE(turkenh): We only set required spec and expect optional ones
			// will properly be initialized with CRD level default values.
			SecretStoreConfig: xpv1.SecretStoreConfig{
				DefaultScope: so.namespace,
			},
		},
	}
	return errors.Wrap(resource.Ignore(kerrors.IsAlreadyExists, kube.Create(ctx, sc)), errCreateDefaultStoreConfig)
}

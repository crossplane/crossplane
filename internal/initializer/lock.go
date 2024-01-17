// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	errApplyLock = "cannot apply lock object"
)

// NewLockObject returns a new *LockObject initializer.
func NewLockObject() *LockObject {
	return &LockObject{}
}

// LockObject has the initializer for creating the Lock object.
type LockObject struct{}

// Run makes sure Lock object exists.
func (lo *LockObject) Run(ctx context.Context, kube client.Client) error {
	l := &v1beta1.Lock{
		ObjectMeta: metav1.ObjectMeta{
			Name: "lock",
		},
	}
	return errors.Wrap(resource.NewAPIPatchingApplicator(kube).Apply(ctx, l), errApplyLock)
}

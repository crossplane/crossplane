/*
Copyright 2021 The Crossplane Authors.

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

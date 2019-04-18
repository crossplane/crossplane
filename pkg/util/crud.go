/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// https://github.com/kubernetes-sigs/controller-runtime/blob/6100e07/pkg/controller/controllerutil/controllerutil.go#L117
//
// This file contains a fork of the above function. At the time of writing the
// latest release of controller-runtime is v0.1.10, which contains a buggy
// CreateOrUpdate implementation.
// TODO(negz): Revert to mainline CreateOrUpdate once we're running v0.2.0 or
// higher per https://github.com/crossplaneio/crossplane/issues/426.

// CreateOrUpdate creates or updates the given object in the Kubernetes
// cluster. The object's desired state must be reconciled with the existing
// state inside the passed in callback MutateFn. The MutateFn is called
// regardless of creating or updating an object.
func CreateOrUpdate(ctx context.Context, c client.Client, obj runtime.Object, f MutateFn) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return errors.Wrap(err, "cannot get object key")
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "could not get object")
		}
		if err := f(); err != nil {
			return errors.Wrap(err, "could not mutate object for creation")
		}
		if err := c.Create(ctx, obj); err != nil {
			return errors.Wrap(err, "could not create object")
		}
		return nil
	}

	existing := obj.DeepCopyObject()
	if err := f(); err != nil {
		return errors.Wrap(err, "could not mutate object for update")
	}

	if reflect.DeepEqual(existing, obj) {
		return nil
	}

	return errors.Wrap(c.Update(ctx, obj), "could not update object")
}

// MutateFn is a function which mutates the existing object into its desired
// state.
type MutateFn func() error

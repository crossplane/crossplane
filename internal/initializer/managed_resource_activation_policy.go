/*
Copyright 2025 The Crossplane Authors.

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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// DefaultManagedResourceActivationPolicy creates a "default" ManagedResourceActivationPolicy
// object. It is a no-op if the object already exists.
func DefaultManagedResourceActivationPolicy(activations ...v1alpha1.ActivationPolicy) StepFunc {
	if len(activations) == 0 {
		return nil
	}
	return func(ctx context.Context, kube client.Client) error {
		rc := &v1alpha1.ManagedResourceActivationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
			Spec: v1alpha1.ManagedResourceActivationPolicySpec{
				Activations: activations,
			},
		}

		return errors.Wrap(resource.Ignore(kerrors.IsAlreadyExists, kube.Create(ctx, rc)), "cannot create ManagedResourceActivationPolicy \"default\"")
	}
}

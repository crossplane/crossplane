/*
Copyright 2026 The Crossplane Authors.

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

// Package ownership provides apply options that require existing ownership.
package ownership

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// MustBeControlledBy requires the current object's controller reference to match
// owner's non-empty UID. Objects with no controller reference are rejected.
// The applicator creates new objects without calling this option.
func MustBeControlledBy(owner metav1.Object) resource.ApplyOption {
	return func(_ context.Context, current, _ runtime.Object) error {
		m, err := meta.Accessor(current)
		if err != nil {
			return err
		}
		if owner.GetUID() == "" || !metav1.IsControlledBy(m, owner) {
			return errors.Errorf("cannot update existing object %q because it is not controlled by %q; check which package or controller created it", m.GetName(), owner.GetName())
		}
		return nil
	}
}

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

// Package names implements name generator
package names

import (
	"context"
	"fmt"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	errGenerateName = "cannot generate a name for a resource"
)

// A NameGenerator finds a free/available name for a resource with a
// specified metadata.generateName value. The name is temporary available,
// but might be taken by the time the resource is created.
type NameGenerator interface {
	GenerateName(ctx context.Context, cd resource.Object) error
}

// A NameGeneratorFn is a function that satisfies NameGenerator.
type NameGeneratorFn func(ctx context.Context, cd resource.Object) error

// GenerateName generates a name using the same algorithm as the API server, and
// verifies temporary name availability. It does not submit the
// resource to the API server and hence it does not fall over validation errors.
func (fn NameGeneratorFn) GenerateName(ctx context.Context, cd resource.Object) error {
	return fn(ctx, cd)
}

// nameGenerator generates a name using the same algorithm as the API
// server and verifies temporary name availability via the API.
type nameGenerator struct {
	reader client.Reader
	namer  names.NameGenerator
}

// NewNameGenerator returns a new NameGenerator.
func NewNameGenerator(c client.Client) NameGenerator {
	return &nameGenerator{reader: c, namer: names.SimpleNameGenerator}
}

func (r *nameGenerator) isAvailable(ctx context.Context, cd resource.Object, name string) (bool, error) {
	obj := composite.Unstructured{}
	obj.SetGroupVersionKind(cd.GetObjectKind().GroupVersionKind())

	err := r.reader.Get(ctx, client.ObjectKey{Name: name, Namespace: cd.GetNamespace()}, &obj)
	if kerrors.IsNotFound(err) {
		return true, nil
	}

	return false, err
}

// GenerateName generates a name using the same algorithm as the API server, and
// verifies temporary name availability. It does not submit the resource
// to the API server and hence it does not fall over validation errors.
func (r *nameGenerator) GenerateName(ctx context.Context, cd resource.Object) error {
	// Don't rename.
	if cd.GetName() != "" || cd.GetGenerateName() == "" {
		return nil
	}

	// If we find the right information on the resource, try once.
	cName := xcrd.GetCompositionResourceName(cd)
	if cName != "" {
		owner := metav1.GetControllerOf(cd)
		if owner != nil && owner.UID != "" {
			// We are going to roll the dice and hope no other XR child has a
			// parent with the same uid ending in an effort to shorten the
			// child name before the ChildName method has to trunc/add a hash.
			uidParts := strings.Split(string(owner.UID), "-")
			uidPart := uidParts[len(uidParts)-1]
			name := ChildName(fmt.Sprintf("%s%s", cd.GetGenerateName(), uidPart), fmt.Sprintf("-%s", cName))
			cd.SetName(name)
			return nil
		}
	}
	// Fallback to a random name.

	// We guess a random name and verify that it is available. Names can become
	// unavailable shortly after. We accept that very little risk of a name collision though:
	// 1. with 8 million names, a collision against 10k names is 0.01%. We retry
	//    name generation 10 times, to reduce the risks to 0.01%^10, which is
	//    acceptable.
	// 2. the risk that a name gets taken between the client.Get and the
	//    client.Create is that of a name conflict between objects just created
	//    in that short time-span. There are 8 million (minus 10k) free names.
	//    And if there are 100 objects created in parallel, chance of conflict
	//    is 0.06% (birthday paradoxon). This is the best we can do here
	//    locally. To reduce that risk even further the caller must employ a
	//    conflict recovery mechanism.
	maxTries := 10
	for range maxTries {
		name := r.namer.GenerateName(cd.GetGenerateName())
		if available, err := r.isAvailable(ctx, cd, name); err != nil {
			return err
		} else if available {
			// The name is available.
			cd.SetName(name)
			return nil
		}
	}

	return errors.New(errGenerateName)
}

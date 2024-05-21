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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
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

// GenerateName generates a name using the same algorithm as the API server, and
// verifies temporary name availability. It does not submit the resource
// to the API server and hence it does not fall over validation errors.
func (r *nameGenerator) GenerateName(ctx context.Context, cd resource.Object) error {
	// Don't rename.
	if cd.GetName() != "" || cd.GetGenerateName() == "" {
		return nil
	}

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
		obj := composite.Unstructured{}
		obj.SetGroupVersionKind(cd.GetObjectKind().GroupVersionKind())
		err := r.reader.Get(ctx, client.ObjectKey{Name: name}, &obj)
		if kerrors.IsNotFound(err) {
			// The name is available.
			cd.SetName(name)
			return nil
		}
		if err != nil {
			return err
		}
	}

	return errors.New(errGenerateName)
}

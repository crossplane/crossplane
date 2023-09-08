/*
Copyright 2022 The Crossplane Authors.

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

// Package webhook contains utilities for building Kubernetes webhooks.
package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// WithMutationFns allows you to initiate the mutator with given list of mutator
// functions.
func WithMutationFns(fns ...MutateFn) MutatorOption {
	return func(m *Mutator) {
		m.MutationChain = fns
	}
}

// MutatorOption configures given Mutator.
type MutatorOption func(*Mutator)

// MutateFn is a single mutating function that can be used by Mutator.
type MutateFn func(ctx context.Context, obj runtime.Object) error

// NewMutator returns a new instance of Mutator that can be used as CustomDefaulter.
func NewMutator(opts ...MutatorOption) *Mutator {
	m := &Mutator{
		MutationChain: []MutateFn{},
	}
	for _, f := range opts {
		f(m)
	}
	return m
}

// Mutator satisfies CustomDefaulter interface with an ordered MutateFn list.
type Mutator struct {
	MutationChain []MutateFn
}

// Default executes the MutatorFns in given order. Its name might sound misleading
// since defaulting seems to be the first use case used by controller-runtime
// but MutatorFns can make any changes on given resource.
func (m *Mutator) Default(ctx context.Context, obj runtime.Object) error {
	for _, f := range m.MutationChain {
		if err := f(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

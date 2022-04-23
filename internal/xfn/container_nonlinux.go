//go:build !linux

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

package xfn

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
)

const errLinuxOnly = "containerized functions are only supported on Linux"

// HasCapSetUID returns false on non-Linux.
func HasCapSetUID() bool { return false }

// HasCapSetGID returns false on non-Linux.
func HasCapSetGID() bool { return false }

// Run returns an error on non-Linux.
func (r *ContainerRunner) Run(_ context.Context, _ *fnv1alpha1.FunctionIO) (*fnv1alpha1.FunctionIO, error) {
	return nil, errors.New(errLinuxOnly)
}

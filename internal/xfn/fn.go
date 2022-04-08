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

	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
)

// A Runner runs an XRM function. It passes the supplied ResourceList as input
// to the function, which should return it with possible mutations.
// A Runner should return an error only if there is an issue executing the
// function. If the function itself encounters an error it should set the
// ResourceList's result field accordingly.
type Runner interface {
	Run(ctx context.Context, in *fnv1alpha1.ResourceList) (*fnv1alpha1.ResourceList, error)
}

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

package render

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// AnnotationKeyRuntime can be added to a Function to control what runtime is
// used to run it locally.
const AnnotationKeyRuntime = "render.crossplane.io/runtime"

// RuntimeType is a type of Function runtime.
type RuntimeType string

// Supported runtimes.
const (
	// The Docker runtime uses a Docker daemon to run a Function. It uses the
	// standard DOCKER_ environment variables to determine how to connect to the
	// daemon.
	AnnotationValueRuntimeDocker RuntimeType = "Docker"

	// The Development runtime expects you to deploy a Function locally. This is
	// mostly useful when developing a Function. The Function must be running
	// with the --insecure flag, i.e. without transport security.
	AnnotationValueRuntimeDevelopment RuntimeType = "Development"

	AnnotationValueRuntimeDefault = AnnotationValueRuntimeDocker
)

// A Runtime runs a Function.
type Runtime interface {
	// Start the Function.
	Start(ctx context.Context) (RuntimeContext, error)
}

// RuntimeContext contains context on how a Function is being run.
type RuntimeContext struct {
	// Target for RunFunctionRequest gRPCs.
	Target string

	// Stop the running Function.
	Stop func(context.Context) error
}

// GetRuntime for the supplied Function, per its annotations.
func GetRuntime(fn pkgv1.Function, log logging.Logger) (Runtime, error) {
	switch r := RuntimeType(fn.GetAnnotations()[AnnotationKeyRuntime]); r {
	case AnnotationValueRuntimeDocker, "":
		return GetRuntimeDocker(fn, log)
	case AnnotationValueRuntimeDevelopment:
		return GetRuntimeDevelopment(fn, log), nil
	default:
		return nil, errors.Errorf("unsupported %q annotation value %q (unknown runtime)", AnnotationKeyRuntime, r)
	}
}

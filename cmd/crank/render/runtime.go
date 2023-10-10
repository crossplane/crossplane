package main

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// AnnotationKeyRuntime can be added to a Function to control what runtime is
// used to run it locally.
const AnnotationKeyRuntime = "xrender.crossplane.io/runtime"

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
func GetRuntime(fn pkgv1beta1.Function) (Runtime, error) {
	switch r := RuntimeType(fn.GetAnnotations()[AnnotationKeyRuntime]); r {
	case AnnotationValueRuntimeDocker, "":
		return GetRuntimeDocker(fn)
	case AnnotationValueRuntimeDevelopment:
		return GetRuntimeDevelopment(fn), nil
	default:
		return nil, errors.Errorf("unsupported %q annotation value %q (unknown runtime)", AnnotationKeyRuntime, r)
	}
}

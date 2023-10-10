package main

import (
	"context"

	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// Annotations that can be used to configure the Development runtime.
const (
	// AnnotationKeyRuntimeDevelopmentTarget can be used to configure the gRPC
	// target where the Function is listening. The default is localhost:9443.
	AnnotationKeyRuntimeDevelopmentTarget = "xrender.crossplane.io/runtime-development-target"
)

// RuntimeDevelopment is largely a no-op. It expects you to run the Function
// manually. This is useful for developing Functions.
type RuntimeDevelopment struct {
	// Target is the gRPC target for the running function, for example
	// localhost:9443.
	Target string
}

// GetRuntimeDevelopment extracts RuntimeDevelopment configuration from the
// supplied Function.
func GetRuntimeDevelopment(fn pkgv1beta1.Function) *RuntimeDevelopment {
	r := &RuntimeDevelopment{Target: "localhost:9443"}
	if t := fn.GetAnnotations()[AnnotationKeyRuntimeDevelopmentTarget]; t != "" {
		r.Target = t
	}
	return r
}

var _ Runtime = &RuntimeDevelopment{}

// Start does nothing. It returns a Stop function that also does nothing.
func (r *RuntimeDevelopment) Start(_ context.Context) (RuntimeContext, error) {
	return RuntimeContext{Target: r.Target, Stop: func(_ context.Context) error { return nil }}, nil
}

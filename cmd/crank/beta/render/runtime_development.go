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

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// Annotations that can be used to configure the Development runtime.
const (
	// AnnotationKeyRuntimeDevelopmentTarget can be used to configure the gRPC
	// target where the Function is listening. The default is localhost:9443.
	AnnotationKeyRuntimeDevelopmentTarget = "render.crossplane.io/runtime-development-target"
)

// RuntimeDevelopment is largely a no-op. It expects you to run the Function
// manually. This is useful for developing Functions.
type RuntimeDevelopment struct {
	// Target is the gRPC target for the running function, for example
	// localhost:9443.
	Target string

	// Function is the name of the function to be run.
	Function string

	// log is the logger for this runtime.
	log logging.Logger
}

// GetRuntimeDevelopment extracts RuntimeDevelopment configuration from the
// supplied Function.
func GetRuntimeDevelopment(fn pkgv1.Function, log logging.Logger) *RuntimeDevelopment {
	r := &RuntimeDevelopment{Target: "localhost:9443", Function: fn.GetName(), log: log}
	if t := fn.GetAnnotations()[AnnotationKeyRuntimeDevelopmentTarget]; t != "" {
		r.Target = t
	}
	return r
}

var _ Runtime = &RuntimeDevelopment{}

// Start does nothing. It returns a Stop function that also does nothing.
func (r *RuntimeDevelopment) Start(_ context.Context) (RuntimeContext, error) {
	r.log.Debug("Starting development runtime. Remember to run the function manually.", "function", r.Function, "target", r.Target)
	return RuntimeContext{Target: r.Target, Stop: func(_ context.Context) error { return nil }}, nil
}

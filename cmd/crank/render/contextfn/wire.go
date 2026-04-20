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

package contextfn

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

// Mirror the render package's runtime-selection annotations. Duplicated here
// rather than importing cmd/crank/render to avoid an import cycle.
const (
	annotationKeyRuntime            = "render.crossplane.io/runtime"
	annotationValueRuntimeInProcess = "InProcess"
)

// Function returns the Function definition the caller must add to the
// render engine's functions list so the in-process context function is
// known by name.
func (h *Handle) Function() pkgv1.Function {
	return pkgv1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:        FunctionName,
			Annotations: map[string]string{annotationKeyRuntime: annotationValueRuntimeInProcess},
		},
	}
}

// CompositeSeedStep returns a Composition pipeline step that seeds the
// pipeline context from the handle's context data. Prepend it to a
// Composition pipeline.
func (h *Handle) CompositeSeedStep() apiextensionsv1.PipelineStep {
	return apiextensionsv1.PipelineStep{
		Step:        stepSeed,
		FunctionRef: apiextensionsv1.FunctionReference{Name: FunctionName},
		Input:       h.seedInput,
	}
}

// CompositeCaptureStep returns a Composition pipeline step that captures
// the end-of-pipeline context into the handle. Append it to a Composition
// pipeline.
func (h *Handle) CompositeCaptureStep() apiextensionsv1.PipelineStep {
	return apiextensionsv1.PipelineStep{
		Step:        stepCapture,
		FunctionRef: apiextensionsv1.FunctionReference{Name: FunctionName},
		Input:       h.captureInput,
	}
}

// OperationSeedStep is the Operation pipeline equivalent of
// CompositeSeedStep.
func (h *Handle) OperationSeedStep() opsv1alpha1.PipelineStep {
	return opsv1alpha1.PipelineStep{
		Step:        stepSeed,
		FunctionRef: opsv1alpha1.FunctionReference{Name: FunctionName},
		Input:       h.seedInput,
	}
}

// OperationCaptureStep is the Operation pipeline equivalent of
// CompositeCaptureStep.
func (h *Handle) OperationCaptureStep() opsv1alpha1.PipelineStep {
	return opsv1alpha1.PipelineStep{
		Step:        stepCapture,
		FunctionRef: opsv1alpha1.FunctionReference{Name: FunctionName},
		Input:       h.captureInput,
	}
}

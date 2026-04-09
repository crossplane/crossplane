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

package render

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/version"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// DefaultCrossplaneImage is the default Crossplane image used for rendering.
const DefaultCrossplaneImage = "xpkg.crossplane.io/crossplane/crossplane"

// An Engine executes a crossplane internal render request and returns the
// response.
type Engine interface {
	// Setup performs engine-specific pre-render preparation, such as
	// creating Docker networks and annotating functions so their containers
	// can reach the render engine. It may mutate fns. The returned cleanup
	// function must be called when rendering is done.
	Setup(ctx context.Context, fns []pkgv1.Function) (cleanup func(), err error)

	// Render executes the render request and returns the response.
	Render(ctx context.Context, req *renderv1alpha1.RenderRequest) (*renderv1alpha1.RenderResponse, error)
}

// EngineFlags contains flags for configuring the render engine. It is embedded
// by render command structs to provide shared engine configuration.
type EngineFlags struct {
	CrossplaneVersion string `help:"Version of the Crossplane image to use for rendering (e.g. v2.2.1). Defaults to the current CLI version." placeholder:"VERSION" xor:"crossplane-selector"`
	CrossplaneImage   string `help:"Override the full Crossplane Docker image reference for rendering."                                       placeholder:"IMAGE"   xor:"crossplane-selector"`
	CrossplaneBinary  string `help:"Path to a local crossplane binary to use instead of Docker."                                              placeholder:"PATH"    type:"existingfile"       xor:"crossplane-selector"`
}

// NewEngine creates an Engine from the flag configuration. If a binary path
// is set, it returns a local engine. Otherwise it returns a Docker engine
// using the resolved image reference.
func (f *EngineFlags) NewEngine(log logging.Logger) Engine {
	if f.CrossplaneBinary != "" {
		return &localRenderEngine{BinaryPath: f.CrossplaneBinary}
	}

	img := f.resolveImage()
	return &dockerRenderEngine{image: img, log: log}
}

func (f *EngineFlags) resolveImage() string {
	if f.CrossplaneImage != "" {
		return f.CrossplaneImage
	}
	tag := f.CrossplaneVersion
	if tag == "" {
		tag = version.New().GetVersionString()
	}
	return fmt.Sprintf("%s:%s", DefaultCrossplaneImage, tag)
}

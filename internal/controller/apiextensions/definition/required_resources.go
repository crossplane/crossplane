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

package definition

import (
	"context"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/engine"
)

// A RequiredResourceWatchStarter starts watches for the resources a composite
// resource's (XR's) function pipeline requires, on a single XR controller. It
// satisfies xfn.RequiredResourceWatcher. The definition reconciler registers one
// per XR controller, so the shared function runner can start required resource
// watches on the right controller when a function's requirements stabilize.
type RequiredResourceWatchStarter struct {
	controllerName string
	engine         composite.WatchStarter
	handler        handler.EventHandler
	log            logging.Logger
}

// NewRequiredResourceWatchStarter creates a RequiredResourceWatchStarter that
// starts watches on the named controller, enqueuing reconciles via the supplied
// handler.
func NewRequiredResourceWatchStarter(name string, e composite.WatchStarter, h handler.EventHandler, log logging.Logger) *RequiredResourceWatchStarter {
	return &RequiredResourceWatchStarter{controllerName: name, engine: e, handler: h, log: log}
}

// WatchRequiredResources ensures the controller is watching each of the supplied
// required resource kinds. StartWatches is idempotent, so it's safe to call on
// every reconcile that resolves requirements; it only starts watches that aren't
// already running.
func (s *RequiredResourceWatchStarter) WatchRequiredResources(ctx context.Context, _ schema.GroupVersionKind, required []schema.GroupVersionKind) error {
	if len(required) == 0 {
		return nil
	}

	ws := make([]engine.Watch, len(required))
	for i, gvk := range required {
		// controller-runtime only supports watching *unstructured.Unstructured,
		// not our wrapper types.
		u := &kunstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		ws[i] = engine.WatchFor(u, engine.WatchTypeRequiredResource, s.handler)
	}

	s.log.Debug("Starting required resource watches", "controller", s.controllerName, "count", len(ws))
	return errors.Wrap(s.engine.StartWatches(ctx, s.controllerName, ws...), "cannot start required resource watches")
}

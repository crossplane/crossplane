/*
Copyright 2025 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
)

// An EventRecorder captures events emitted during reconciliation. It satisfies
// the event.Recorder interface.
type EventRecorder struct {
	events []OutputEvent
}

// Event records an event.
func (r *EventRecorder) Event(_ runtime.Object, e event.Event) {
	r.events = append(r.events, OutputEvent{
		Type:    string(e.Type),
		Reason:  string(e.Reason),
		Message: e.Message,
	})
}

// Events returns all recorded events.
func (r *EventRecorder) Events() []OutputEvent {
	out := make([]OutputEvent, len(r.events))
	copy(out, r.events)
	return out
}

// WithAnnotations returns the same recorder. Annotation support is not needed
// for render output.
func (r *EventRecorder) WithAnnotations(_ ...string) event.Recorder {
	return r
}

/*
Copyright 2019 The Crossplane Authors.

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

// Package event records Kubernetes events.
package event

import (
	"maps"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// A Type of event.
type Type string

// Event types. See below for valid types.
// https://godoc.org/k8s.io/client-go/tools/record#EventRecorder
const (
	TypeNormal  Type = "Normal"
	TypeWarning Type = "Warning"
)

// Reason an event occurred.
type Reason string

// An Event relating to a Crossplane resource.
type Event struct {
	Type        Type
	Reason      Reason
	Message     string
	Annotations map[string]string
}

// Normal returns a normal, informational event.
func Normal(r Reason, message string, keysAndValues ...string) Event {
	e := Event{
		Type:        TypeNormal,
		Reason:      r,
		Message:     message,
		Annotations: map[string]string{},
	}
	sliceMap(keysAndValues, e.Annotations)

	return e
}

// Warning returns a warning event, typically due to an error.
func Warning(r Reason, err error, keysAndValues ...string) Event {
	e := Event{
		Type:        TypeWarning,
		Reason:      r,
		Message:     err.Error(),
		Annotations: map[string]string{},
	}
	sliceMap(keysAndValues, e.Annotations)

	return e
}

// A Recorder records Kubernetes events.
type Recorder interface {
	Event(obj runtime.Object, e Event)
	WithAnnotations(keysAndValues ...string) Recorder
}

// An APIRecorder records Kubernetes events to an API server.
type APIRecorder struct {
	kube        record.EventRecorder
	annotations map[string]string
	filterFns   []FilterFn
}

// FilterFn is a function used to filter events.
// It should return false when events should not be sent.
type FilterFn func(obj runtime.Object, e Event) bool

// NewAPIRecorder returns an APIRecorder that records Kubernetes events to an
// APIServer using the supplied EventRecorder.
func NewAPIRecorder(r record.EventRecorder, fns ...FilterFn) *APIRecorder {
	return &APIRecorder{kube: r, annotations: map[string]string{}, filterFns: fns}
}

// Event records the supplied event.
func (r *APIRecorder) Event(obj runtime.Object, e Event) {
	for _, filter := range r.filterFns {
		if filter(obj, e) {
			return
		}
	}

	r.kube.AnnotatedEventf(obj, r.annotations, string(e.Type), string(e.Reason), "%s", e.Message)
}

// WithAnnotations returns a new *APIRecorder that includes the supplied
// annotations with all recorded events.
func (r *APIRecorder) WithAnnotations(keysAndValues ...string) Recorder {
	ar := NewAPIRecorder(r.kube)
	maps.Copy(ar.annotations, r.annotations)

	sliceMap(keysAndValues, ar.annotations)

	return ar
}

func sliceMap(from []string, to map[string]string) {
	for i := 0; i+1 < len(from); i += 2 {
		k, v := from[i], from[i+1]
		to[k] = v
	}
}

// A NopRecorder does nothing.
type NopRecorder struct{}

// NewNopRecorder returns a Recorder that does nothing.
func NewNopRecorder() *NopRecorder {
	return &NopRecorder{}
}

// Event does nothing.
func (r *NopRecorder) Event(_ runtime.Object, _ Event) {}

// WithAnnotations does nothing.
func (r *NopRecorder) WithAnnotations(_ ...string) Recorder { return r }

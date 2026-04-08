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

// A FunctionInput identifies a running function by name and gRPC address.
type FunctionInput struct {
	// Name of the function, matching the pipeline step's functionRef.name.
	Name string `json:"name" yaml:"name"`
	// Address is the gRPC address of the running function (e.g.
	// "localhost:9443").
	Address string `json:"address" yaml:"address"`
}

// An OutputEvent represents a Kubernetes event the reconciler would emit.
type OutputEvent struct {
	// Type is Normal or Warning.
	Type string `json:"type" yaml:"type"`
	// Reason is the short, machine-readable reason for the event.
	Reason string `json:"reason" yaml:"reason"`
	// Message is the human-readable description of the event.
	Message string `json:"message" yaml:"message"`
}

/*
Copyright 2018 The Crossplane Authors.

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

// Package log provides a logger that satisfies https://github.com/go-logr/logr.
// It is implemented as a light wrapper around sigs.k8s.io/controller-runtime/pkg/log
package log

import (
	"github.com/go-logr/logr"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log = runtimelog.NewDelegatingLogger(runtimelog.NullLogger{})

	// Log is the base logger used by Crossplane. It delegates to another
	// logr.Logger. You *must* call SetLogger to get any actual logging.
	Log = log.WithName("crossplane")
)

// SetLogger sets a concrete logging implementation for all deferred Loggers.
func SetLogger(l logr.Logger) {
	log.Fulfill(l)
}

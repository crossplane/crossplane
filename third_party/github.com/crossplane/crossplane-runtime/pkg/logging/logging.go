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

// Package logging provides Crossplane's recommended logging interface.
//
// The logging interface defined by this package is inspired by the following:
//
// * https://peter.bourgon.org/go-best-practices-2016/#logging-and-instrumentation
// * https://dave.cheney.net/2015/11/05/lets-talk-about-logging
// * https://dave.cheney.net/2017/01/23/the-package-level-logger-anti-pattern
// * https://github.com/crossplane/crossplane/blob/c06433/design/one-pager-error-and-event-reporting.md
//
// It is similar to other logging interfaces inspired by said article, namely:
//
// * https://github.com/go-logr/logr
// * https://github.com/go-log/log
//
// Crossplane prefers not to use go-logr because it desires a simpler API with
// only two levels (per Dave's article); Info and Debug. Crossplane prefers not
// to use go-log because it does not support structured logging. This package
// *is* however a subset of go-logr's functionality, and is intended to wrap
// go-logr (interfaces all the way down!), in order to maintain compatibility
// with the https://github.com/kubernetes-sigs/controller-runtime/ log plumbing.
package logging

import (
	"github.com/go-logr/logr"
)

// A Logger logs messages. Messages may be supplemented by structured data.
type Logger interface {
	// Info logs a message with optional structured data. Structured data must
	// be supplied as an array that alternates between string keys and values of
	// an arbitrary type. Use Info for messages that Crossplane operators are
	// very likely to be concerned with when running Crossplane.
	Info(msg string, keysAndValues ...any)

	// Debug logs a message with optional structured data. Structured data must
	// be supplied as an array that alternates between string keys and values of
	// an arbitrary type. Use Debug for messages that Crossplane operators or
	// developers may be concerned with when debugging Crossplane.
	Debug(msg string, keysAndValues ...any)

	// WithValues returns a Logger that will include the supplied structured
	// data with any subsequent messages it logs. Structured data must
	// be supplied as an array that alternates between string keys and values of
	// an arbitrary type.
	WithValues(keysAndValues ...any) Logger
}

// NewNopLogger returns a Logger that does nothing.
func NewNopLogger() Logger { return nopLogger{} }

type nopLogger struct{}

func (l nopLogger) Info(_ string, _ ...any)    {}
func (l nopLogger) Debug(_ string, _ ...any)   {}
func (l nopLogger) WithValues(_ ...any) Logger { return nopLogger{} }

// NewLogrLogger returns a Logger that is satisfied by the supplied logr.Logger,
// which may be satisfied in turn by various logging implementations (Zap, klog,
// etc). Debug messages are logged at V(1).
func NewLogrLogger(l logr.Logger) Logger {
	return logrLogger{log: l}
}

type logrLogger struct {
	log logr.Logger
}

func (l logrLogger) Info(msg string, keysAndValues ...any) {
	l.log.Info(msg, keysAndValues...) //nolint:logrlint // False positive - logrlint thinks there's an odd number of args.
}

func (l logrLogger) Debug(msg string, keysAndValues ...any) {
	l.log.V(1).Info(msg, keysAndValues...) //nolint:logrlint // False positive - logrlint thinks there's an odd number of args.
}

func (l logrLogger) WithValues(keysAndValues ...any) Logger {
	return logrLogger{log: l.log.WithValues(keysAndValues...)} //nolint:logrlint // False positive - logrlint thinks there's an odd number of args.
}

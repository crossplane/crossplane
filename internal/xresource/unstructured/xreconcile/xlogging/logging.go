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

// Package xlogging contains a context storage for logger.
package xlogging

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

type loggerKey struct{}

// WithLogger returns a copy of parent context in which the
// value associated with logger key is the supplied logger.
func WithLogger(ctx context.Context, logger logging.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the logger stored in context.
// Returns nop logger if no logger is set in context, or if the stored value is
// not of correct type.
func FromContext(ctx context.Context) logging.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(logging.Logger); ok {
		return logger
	}
	return logging.NewNopLogger()
}

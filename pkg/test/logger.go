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

package test

import (
	"testing"

	"github.com/go-logr/logr"
)

// TestLogger is intended to allow a test to reliably capture logger output. This is useful for
// tests which are more on the integration end of the spectrum, which may have complicated behavior
// happening underneath that we want to observe. For example, integration testing with a controller.
// It's mostly copied from logr's test logger implementation.
type TestLogger struct {
	T      *testing.T
	name   string
	values []interface{}
}

// Info logs at the info level.
func (log TestLogger) Info(msg string, args ...interface{}) {
	log.T.Logf("name=%s msg='%s' values=%v args=%v", log.name, msg, log.values, args)
}

// Enabled always returns false
func (TestLogger) Enabled() bool {
	return false
}

// Error logs an error
func (log TestLogger) Error(err error, msg string, args ...interface{}) {
	log.T.Logf("name=%s msg='%s' err='%v' -- values=%v args=%v", log.name, msg, err, log.values, args)
}

// V would normally set the log level, but is a noop in this implementation.
func (log TestLogger) V(v int) logr.InfoLogger {
	return log
}

// WithName sets a name field which will be included in every log line.
func (log TestLogger) WithName(name string) logr.Logger {
	return TestLogger{
		T:      log.T,
		name:   name,
		values: log.values,
	}
}

// WithValues sets values to include in every log line.
func (log TestLogger) WithValues(_ ...interface{}) logr.Logger {
	return TestLogger{
		T:      log.T,
		name:   log.name,
		values: log.values,
	}
}

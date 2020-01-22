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

type TestLogger struct {
	T      *testing.T
	name   string
	values []interface{}
}

func (log TestLogger) Info(msg string, args ...interface{}) {
	log.T.Logf("name=%s msg='%s' values=%v args=%v", log.name, msg, log.values, args)
}

func (_ TestLogger) Enabled() bool {
	return false
}

func (log TestLogger) Error(err error, msg string, args ...interface{}) {
	log.T.Logf("name=%s msg='%s' err='%v' -- values=%v args=%v", log.name, msg, err, log.values, args)
}

func (log TestLogger) V(v int) logr.InfoLogger {
	return log
}

func (log TestLogger) WithName(name string) logr.Logger {
	return TestLogger{
		T:      log.T,
		name:   name,
		values: log.values,
	}
}

func (log TestLogger) WithValues(_ ...interface{}) logr.Logger {
	return TestLogger{
		T:      log.T,
		name:   log.name,
		values: log.values,
	}
}

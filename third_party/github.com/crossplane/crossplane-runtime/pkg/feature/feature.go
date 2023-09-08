/*
Copyright 2021 The Crossplane Authors.

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

// Package feature contains utilities for managing Crossplane features.
package feature

import (
	"sync"
)

// A Flag enables a particular feature.
type Flag string

// Flags that are enabled. The zero value - i.e. &feature.Flags{} - is usable.
type Flags struct {
	m       sync.RWMutex
	enabled map[Flag]bool
}

// Enable a feature flag.
func (fs *Flags) Enable(f Flag) {
	fs.m.Lock()
	if fs.enabled == nil {
		fs.enabled = make(map[Flag]bool)
	}
	fs.enabled[f] = true
	fs.m.Unlock()
}

// Enabled returns true if the supplied feature flag is enabled.
func (fs *Flags) Enabled(f Flag) bool {
	if fs == nil {
		return false
	}
	fs.m.RLock()
	defer fs.m.RUnlock()
	return fs.enabled[f]
}

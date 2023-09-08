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

package v1

import (
	"dario.cat/mergo"
)

// MergeOptions Specifies merge options on a field path
type MergeOptions struct { // TODO(aru): add more options that control merging behavior
	// Specifies that already existing values in a merged map should be preserved
	// +optional
	KeepMapValues *bool `json:"keepMapValues,omitempty"`
	// Specifies that already existing elements in a merged slice should be preserved
	// +optional
	AppendSlice *bool `json:"appendSlice,omitempty"`
}

// MergoConfiguration the default behavior is to replace maps and slices
func (mo *MergeOptions) MergoConfiguration() []func(*mergo.Config) {
	config := []func(*mergo.Config){mergo.WithOverride}
	if mo == nil {
		return config
	}

	if mo.KeepMapValues != nil && *mo.KeepMapValues {
		config = config[:0]
	}
	if mo.AppendSlice != nil && *mo.AppendSlice {
		config = append(config, mergo.WithAppendSlice)
	}
	return config
}

// IsAppendSlice returns true if mo.AppendSlice is set to true
func (mo *MergeOptions) IsAppendSlice() bool {
	return mo != nil && mo.AppendSlice != nil && *mo.AppendSlice
}

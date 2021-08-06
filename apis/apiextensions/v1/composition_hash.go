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
	"fmt"
	"hash/fnv"

	"sigs.k8s.io/yaml"
)

// Hash of the CompositionSpec.
func (cs CompositionSpec) Hash() string {
	h := fnv.New64a()
	y, err := yaml.Marshal(cs)
	if err != nil {
		// I believe this should be impossible given we're marshalling a
		// known, strongly typed struct.
		return "unknown"
	}
	h.Write(y) //nolint:errcheck // Writing to a hash never errors.
	return fmt.Sprintf("%x", h.Sum64())
}

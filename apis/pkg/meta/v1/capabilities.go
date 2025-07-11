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

package v1

// Known function capabilities. This shouldn't be treated as an exhaustive list.
// A package's capabilities array may contain arbitrary entries that aren't
// meaningful to Crossplane but might be meaningful to some other consumer.
const (
	// A function that can be used in a composition.
	FunctionCapabilityComposition = "composition"

	// A function that can be used in an operation.
	FunctionCapabilityOperation = "operation"
)

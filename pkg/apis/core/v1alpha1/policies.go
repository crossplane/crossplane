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

package v1alpha1

// A ReclaimPolicy determines what should happen to managed resources when their
// bound resource claims are deleted.
type ReclaimPolicy string

const (
	// ReclaimDelete means the managed resource will be deleted when its bound
	// resource claim is deleted.
	ReclaimDelete ReclaimPolicy = "Delete"

	// ReclaimRetain means the managed resource will remain when its bound
	// resource claim is deleted.
	ReclaimRetain ReclaimPolicy = "Retain"
)

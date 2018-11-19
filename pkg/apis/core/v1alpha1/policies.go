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

package v1alpha1

// ReclaimPolicy describes a policy for end-of-life maintenance of storage resources
type ReclaimPolicy string

const (
	// ReclaimDelete means the cloud provider resource backing this custom resource (CR) will be deleted upon CR deletion
	ReclaimDelete ReclaimPolicy = "Delete"
	// ReclaimRetain means the cloud provider resource backing this custom resource (CR) will be will be left in its current phase upon CR deletion for manual reclamation by the administrator.
	// The default policy is Retain.
	ReclaimRetain ReclaimPolicy = "Retain"
)

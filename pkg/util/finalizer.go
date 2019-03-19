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

package util

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RemoveFinalizer - from the list of kubernetes object finalizers
func RemoveFinalizer(o v1.Object, finalizer string) {
	finalizers := o.GetFinalizers()
	for index, f := range finalizers {
		if f == finalizer {
			finalizers = append(finalizers[:index], finalizers[index+1:]...)
		}
	}
	o.SetFinalizers(finalizers)
}

// AddFinalizer - add finalizer (if it wasn't added before)
func AddFinalizer(o v1.Object, finalizer string) {
	finalizers := o.GetFinalizers()
	for _, f := range finalizers {
		if f == finalizer {
			return
		}
	}
	o.SetFinalizers(append(finalizers, finalizer))
}

// HasFinalizer - check if given instance has a given finalizer
func HasFinalizer(o v1.Object, finalizer string) bool {
	finalizers := o.GetFinalizers()
	for _, f := range finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}

/*
Copyright 2023 The Crossplane Authors.

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

package errors

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// WrapFieldError wraps the given field.Error adding the given field.Path as root of the Field.
func WrapFieldError(err *field.Error, path *field.Path) *field.Error {
	if err == nil {
		return nil
	}
	if path == nil {
		return err
	}
	err.Field = path.Child(err.Field).String()
	return err
}

// WrapFieldErrorList wraps the given field.ErrorList adding the given field.Path as root of the Field.
func WrapFieldErrorList(errs field.ErrorList, path *field.Path) field.ErrorList {
	if path == nil {
		return errs
	}
	for i := range errs {
		errs[i] = WrapFieldError(errs[i], path)
	}
	return errs
}

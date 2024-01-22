// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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

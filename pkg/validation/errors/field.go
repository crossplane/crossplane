package errors

import "k8s.io/apimachinery/pkg/util/validation/field"

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

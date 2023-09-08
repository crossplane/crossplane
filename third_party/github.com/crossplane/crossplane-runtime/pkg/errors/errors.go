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

// Package errors is a github.com/pkg/errors compatible API for native errors.
// It includes only the subset of the github.com/pkg/errors API that is used by
// the Crossplane project.
package errors

import (
	"errors"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// New returns an error that formats as the given text. Each call to New returns
// a distinct error value even if the text is identical.
func New(text string) error { return errors.New(text) }

// Is reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained
// by repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
//
// An error type might provide an Is method so it can be treated as equivalent
// to an existing error. For example, if MyError defines
//
//	func (m MyError) Is(target error) bool { return target == fs.ErrExist }
//
// then Is(MyError{}, fs.ErrExist) returns true. See syscall.Errno.Is for
// an example in the standard library.
func Is(err, target error) bool { return errors.Is(err, target) }

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true. Otherwise, it returns false.
//
// The chain consists of err itself followed by the sequence of errors obtained
// by repeatedly calling Unwrap.
//
// An error matches target if the error's concrete value is assignable to the
// value pointed to by target, or if the error has a method As(any) bool
// such that As(target) returns true. In the latter case, the As method is
// responsible for setting target.
//
// An error type might provide an As method so it can be treated as if it were a
// different error type.
//
// As panics if target is not a non-nil pointer to either a type that implements
// error, or to any interface type.
func As(err error, target any) bool { return errors.As(err, target) }

// Unwrap returns the result of calling the Unwrap method on err, if err's type
// contains an Unwrap method returning error. Otherwise, Unwrap returns nil.
func Unwrap(err error) error { return errors.Unwrap(err) }

// Errorf formats according to a format specifier and returns the string as a
// value that satisfies error.
//
// If the format specifier includes a %w verb with an error operand, the
// returned error will implement an Unwrap method returning the operand. It is
// invalid to include more than one %w verb or to supply it with an operand that
// does not implement the error interface. The %w verb is otherwise a synonym
// for %v.
func Errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

// WithMessage annotates err with a new message. If err is nil, WithMessage
// returns nil.
func WithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WithMessagef annotates err with the format specifier. If err is nil,
// WithMessagef returns nil.
func WithMessagef(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Wrap is an alias for WithMessage.
func Wrap(err error, message string) error {
	return WithMessage(err, message)
}

// Wrapf is an alias for WithMessagef
func Wrapf(err error, format string, args ...any) error {
	return WithMessagef(err, format, args...)
}

// Cause calls Unwrap on each error it finds. It returns the first error it
// finds that does not have an Unwrap method - i.e. the first error that was not
// the result of a Wrap call, a Wrapf call, or an Errorf call with %w wrapping.
func Cause(err error) error {
	type wrapped interface {
		Unwrap() error
	}

	for err != nil {
		//nolint:errorlint // We actually do want to check the outermost error.
		w, ok := err.(wrapped)
		if !ok {
			return err
		}
		err = w.Unwrap()
	}

	return err
}

// MultiError is an error that wraps multiple errors.
type MultiError interface {
	error
	Unwrap() []error
}

// Join returns an error that wraps the given errors. Any nil error values are
// discarded. Join returns nil if errs contains no non-nil values. The error
// formats as the concatenation of the strings obtained by calling the Error
// method of each element of errs and formatting like:
//
//	[first error, second error, third error]
//
// Note: aggregating errors should not be the default. Usually, return only the
// first error, and only aggregate if there is clear value to the user.
func Join(errs ...error) MultiError {
	err := kerrors.NewAggregate(errs)
	if err == nil {
		return nil
	}
	return multiError{aggregate: err}
}

type multiError struct {
	aggregate kerrors.Aggregate
}

func (m multiError) Error() string {
	return m.aggregate.Error()
}
func (m multiError) Unwrap() []error {
	return m.aggregate.Errors()
}

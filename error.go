// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dig

import (
	"fmt"

	"go.uber.org/dig/internal/digreflect"
)

// Errors which know their underlying cause should implement this interface to
// be compatible with RootCause.
//
// We use an unexported "cause" method instead of "Cause" because we don't
// want dig-internal causes to be confused with the cause of the user-provided
// errors. (For example, if the users are using github.com/pkg/errors.)
type causer interface {
	cause() error
}

// RootCause returns the original error that caused the provided dig failure.
//
// RootCause may be used on errors returned by Invoke to get the original
// error returned by a constructor or invoked function.
func RootCause(err error) error {
	for {
		if e, ok := err.(causer); ok {
			err = e.cause()
		} else {
			return err
		}
	}
}

// errWrapf wraps an existing error with more contextual information.
//
// The given error is treated as the cause of the returned error (see causer).
//
//   RootCause(errWrapf(errWrapf(err, ...), ...)) == err
//
// Use errWrapf instead of fmt.Errorf if the message ends with ": <original error>".
func errWrapf(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	return wrappedError{err: err, msg: msg}
}

type wrappedError struct {
	err error
	msg string
}

func (e wrappedError) cause() error { return e.err }

func (e wrappedError) Error() string {
	return fmt.Sprintf("%v: %v", e.msg, e.err)
}

// errProvide is returned when a constructor could not be Provided into the
// container.
type errProvide struct {
	Func   *digreflect.Func
	Reason error
}

func (e errProvide) cause() error { return e.Reason }

func (e errProvide) Error() string {
	return fmt.Sprintf("function %v cannot be provided: %v", e.Func, e.Reason)
}

// errConstructorFailed is returned when a user-provided constructor failed
// with a non-nil error.
type errConstructorFailed struct {
	Func   *digreflect.Func
	Reason error
}

func (e errConstructorFailed) cause() error { return e.Reason }

func (e errConstructorFailed) Error() string {
	return fmt.Sprintf("function %v returned a non-nil error: %v", e.Func, e.Reason)
}

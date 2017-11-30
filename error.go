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
	"bytes"
	"fmt"
	"reflect"

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

// errArgumentsFailed is returned when a function could not be run because one
// of its dependencies failed to build for any reason.
type errArgumentsFailed struct {
	Func   *digreflect.Func
	Reason error
}

func (e errArgumentsFailed) cause() error { return e.Reason }

func (e errArgumentsFailed) Error() string {
	return fmt.Sprintf("could not build arguments for function %v: %v", e.Func, e.Reason)
}

// errParamSingleFailed is returned when a paramSingle could not be built.
type errParamSingleFailed struct {
	Key    key
	Reason error
}

func (e errParamSingleFailed) cause() error { return e.Reason }

func (e errParamSingleFailed) Error() string {
	return fmt.Sprintf("failed to build %v: %v", e.Key, e.Reason)
}

// errParamGroupFailed is returned when a value group cannot be built because
// any of the values in the group failed to build.
type errParamGroupFailed struct {
	Key    key
	Reason error
}

func (e errParamGroupFailed) cause() error { return e.Reason }

func (e errParamGroupFailed) Error() string {
	return fmt.Sprintf("could not build value group %v: %v", e.Key, e.Reason)
}

// errMissingType is returned when a single value that was expected in the
// container was not available.
type errMissingType struct {
	Key key

	// If set, we'll include a suggestion of what the user probably meant.
	//
	// So in addition to "cannot find X", we can tell them "did you mean *X?".
	Typo *key
}

func newErrMissingType(c *Container, k key) errMissingType {
	// If the type being asked for is the pointer that is not found, check if
	// the graph contains the value type element - perhaps the user
	// accidentally included a splat and vice versa.
	var typo reflect.Type
	if k.t.Kind() == reflect.Ptr {
		typo = k.t.Elem()
	} else {
		typo = reflect.PtrTo(k.t)
	}

	// TODO(abg): If the requested type is an interface, look for
	// implementations of that interface.

	err := errMissingType{Key: k}

	typoK := k
	typoK.t = typo
	if len(c.providers[typoK]) > 0 {
		err.Typo = &typoK
	}

	return err
}

func (e errMissingType) Error() string {
	// Sample messages:
	//
	//   type io.Reader is not in the container, did you mean to Provide it?
	//   type bytes.Buffer is not in the container, did you mean to use *bytes.Buffer?
	//   type *foo[name="bar"] is not in the container, did you mean to use foo[name="bar"]?

	b := new(bytes.Buffer)

	fmt.Fprintf(b, "type %v is not in the container", e.Key)

	if e.Typo != nil {
		fmt.Fprintf(b, ", did you mean to use %v?", *e.Typo)
	} else {
		fmt.Fprint(b, ", did you mean to Provide it?")
	}

	return b.String()
}

// errMissingManyTypes combines multiple errMissingType errors.
type errMissingManyTypes []errMissingType // length must be non-zero

func (e errMissingManyTypes) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}

	b := new(bytes.Buffer)

	b.WriteString("the following types are not in the container: ")
	for i, err := range e {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(b, "%v", err.Key)
		if err.Typo != nil {
			fmt.Fprintf(b, " (did you mean %v?)", *err.Typo)
		}
	}

	return b.String()
}

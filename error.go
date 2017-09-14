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

import "fmt"

// errRootCause returns the root cause of the provided error.
//
// Returns the error as-is if no root cause is known.
func errRootCause(err error) error {
	if we, ok := err.(wrappedError); ok {
		return we.rootCause
	}
	return err
}

// errWrapf wraps an existing error with more contextual information.
//
// The message for the returned error is the provided error prepended with the
// provided message, separated by a ":".
//
// The given error is treated as the root cause of the returned error,
// retrievable by using errRootCause. If the provided error knew its root
// cause, that knowledge is retained in the returned error.
//
//   errRootCaus(errWrapf(errWrapf(err, ...), ...)) == err
//
// Use errWrapf in the rest of dig in place of fmt.Errorf if the message ends
// with ": <original error>".
func errWrapf(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	rootCause := err
	if we, ok := err.(wrappedError); ok {
		rootCause = we.rootCause
	}

	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	return wrappedError{
		rootCause: rootCause,
		err:       fmt.Errorf("%v: %v", msg, err),
	}
}

// wrappedError is a wrapper around error that tracks the root cause of the
// error.
//
// The root cause will be retained between errWrapf calls and retrievable by
// using errRootCause.
type wrappedError struct {
	rootCause error
	err       error
}

func (e wrappedError) Error() string {
	return e.err.Error()
}

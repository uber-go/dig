// Copyright (c) 2019 Uber Technologies, Inc.
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
	"sort"

	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/dot"
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

// errMissingDependencies is returned when the dependencies of a function are
// not available in the container.
type errMissingDependencies struct {
	Func   *digreflect.Func
	Reason error
}

func (e errMissingDependencies) cause() error { return e.Reason }

func (e errMissingDependencies) Error() string {
	return fmt.Sprintf("missing dependencies for function %v: %v", e.Func, e.Reason)
}

// errParamSingleFailed is returned when a paramSingle could not be built.
type errParamSingleFailed struct {
	Key    key
	Reason error
	CtorID dot.CtorID
}

func (e errParamSingleFailed) cause() error { return e.Reason }

func (e errParamSingleFailed) Error() string {
	return fmt.Sprintf("failed to build %v: %v", e.Key, e.Reason)
}

func (e errParamSingleFailed) updateGraph(g *dot.Graph) {
	failed := &dot.Result{
		Node: &dot.Node{
			Name:  e.Key.name,
			Group: e.Key.group,
			Type:  e.Key.t,
		},
	}
	g.FailNodes([]*dot.Result{failed}, e.CtorID)
}

// errParamGroupFailed is returned when a value group cannot be built because
// any of the values in the group failed to build.
type errParamGroupFailed struct {
	Key    key
	Reason error
	CtorID dot.CtorID
}

func (e errParamGroupFailed) cause() error { return e.Reason }

func (e errParamGroupFailed) Error() string {
	return fmt.Sprintf("could not build value group %v: %v", e.Key, e.Reason)
}

func (e errParamGroupFailed) updateGraph(g *dot.Graph) {
	g.FailGroupNodes(e.Key.group, e.Key.t, e.CtorID)
}

// errMissingType is returned when a single value that was expected in the
// container was not available.
type errMissingType struct {
	Key key

	// If non-empty, we will include suggestions for what the user may have
	// meant.
	suggestions []key
}

func newErrMissingType(c containerStore, k key) errMissingType {
	// Possible types we will look for in the container. We will always look
	// for pointers to the requested type and some extras on a per-Kind basis.

	suggestions := []reflect.Type{reflect.PtrTo(k.t)}
	if k.t.Kind() == reflect.Ptr {
		// The user requested a pointer but maybe we have a value.
		suggestions = append(suggestions, k.t.Elem())
	}

	knownTypes := c.knownTypes()
	if k.t.Kind() == reflect.Interface {
		// Maybe we have an implementation of the interface.
		for _, t := range knownTypes {
			if t.Implements(k.t) {
				suggestions = append(suggestions, t)
			}
		}
	} else {
		// Maybe we have an interface that this type implements.
		for _, t := range knownTypes {
			if t.Kind() == reflect.Interface {
				if k.t.Implements(t) {
					suggestions = append(suggestions, t)
				}
			}
		}
	}

	// range through c.providers is non-deterministic. Let's sort the list of
	// suggestions.
	sort.Sort(byTypeName(suggestions))

	err := errMissingType{Key: k}
	for _, t := range suggestions {
		if len(c.getValueProviders(k.name, t)) > 0 {
			k.t = t
			err.suggestions = append(err.suggestions, k)
		}
	}

	return err
}

func (e errMissingType) Error() string {
	// Sample messages:
	//
	//   type io.Reader is not in the container, did you mean to Provide it?
	//   type io.Reader is not in the container, did you mean to use one of *bytes.Buffer, *MyBuffer
	//   type bytes.Buffer is not in the container, did you mean to use *bytes.Buffer?
	//   type *foo[name="bar"] is not in the container, did you mean to use foo[name="bar"]?

	b := new(bytes.Buffer)

	fmt.Fprintf(b, "type %v is not in the container", e.Key)
	switch len(e.suggestions) {
	case 0:
		b.WriteString(", did you mean to Provide it?")
	case 1:
		fmt.Fprintf(b, ", did you mean to use %v?", e.suggestions[0])
	default:
		b.WriteString(", did you mean to use one of ")
		for i, k := range e.suggestions {
			if i > 0 {
				b.WriteString(", ")
				if i == len(e.suggestions)-1 {
					b.WriteString("or ")
				}
			}
			fmt.Fprint(b, k)
		}
		b.WriteString("?")
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
		switch len(err.suggestions) {
		case 0:
			// do nothing
		case 1:
			fmt.Fprintf(b, " (did you mean %v?)", err.suggestions[0])
		default:
			b.WriteString(" (did you mean ")
			for i, k := range err.suggestions {
				if i > 0 {
					b.WriteString(", ")
					if i == len(err.suggestions)-1 {
						b.WriteString("or ")
					}
				}
				fmt.Fprint(b, k)
			}
			b.WriteString("?)")
		}
	}

	return b.String()
}

func (e errMissingManyTypes) updateGraph(g *dot.Graph) {
	missing := make([]*dot.Result, len(e))

	for i, err := range e {
		missing[i] = &dot.Result{
			Node: &dot.Node{
				Name:  err.Key.name,
				Group: err.Key.group,
				Type:  err.Key.t,
			},
		}
	}
	g.AddMissingNodes(missing)
}

type errVisualizer interface {
	updateGraph(*dot.Graph)
}

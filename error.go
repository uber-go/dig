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
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"

	"go.uber.org/dig/internal/digerror"
	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/dot"
)

// All dig errors should implement this interface so users can follow standard
// errors.Is & errors.As API to check if an error comes from Dig
type DigError interface {
	error
	dummy()
}

type defaultError struct {
	msg string
}

// A default error type that implements DigError for simple errors
var _ DigError = defaultError{}

func (e defaultError) dummy() {}

func (e defaultError) Error() string {
	return e.msg
}

func newError(msg string) DigError {
	return defaultError{msg}
}

// errSpecification is returned whenever the user provides bad input when
// interacting with the container. May optionally have a more detailed
// error wrapped underneath.
// TODO invalidInputError
type errSpecification struct {
	Message string
	Cause   error
}

// newErrSpecification creates a new errSpecification. If there is no other
// error to wrap, it is safe to pass nil.
func newErrSpecification(msg string, cause error) errSpecification {
	return errSpecification{msg, cause}
}

var _ DigError = errSpecification{}

func (e errSpecification) dummy() {}

func (e errSpecification) Unwrap() error {
	return e.Cause
}

func (e errSpecification) writeMessage(w io.Writer, _ string) {
	fmt.Fprintf(w, e.Message)
}

func (e errSpecification) Error() string { return fmt.Sprint(e) }
func (e errSpecification) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// wrappedError is a interface for wrapped dig errors to implement that allows them
// to be formatted via formatError()
type wrappedError interface {
	DigError
	fmt.Formatter
	Unwrap() error
	writeMessage(w io.Writer, v string)
}

// formatError will call a wrappedError's writeMessage() method to print the error message
// and then will automatically attempt to print errors wrapped underneath (which can create
// a recursive effect if the wrapped error's Format() method points back to this function).
func formatError(e wrappedError, w fmt.State, v rune) {
	multiline := w.Flag('+') && v == 'v'
	verb := "%v"
	if multiline {
		verb = "%+v"
	}

	// "context: " or "context:\n"
	e.writeMessage(w, verb)

	// Will route back to this function recursively if next error
	// is also wrapped and points back here
	wrappedError := errors.Unwrap(e)
	if wrappedError != nil {
		io.WriteString(w, ":")
		if multiline {
			io.WriteString(w, "\n")
		} else {
			io.WriteString(w, " ")
		}
		fmt.Fprintf(w, verb, wrappedError)
	}
}

// RootCause returns the original error that caused the provided dig failure.
//
// RootCause may be used on errors returned by Invoke to get the original
// error returned by a constructor or invoked function.
// If there is no non-dig error in the chain, RootCause will return nil.
func RootCause(err error) error {
	var de DigError
	for {
		if ok := errors.As(err, &de); ok { // This is a dig error
			err = errors.Unwrap(de) // Attempt to unwrap error
		} else { // This is not a dig error
			return err
		}
	}
}

// errf is a version of fmt.Errorf with support for a chain of multiple
// formatted error messages that ensures that any errors that come out
// satisfy DigError.
//
// After msg, N arguments are consumed as formatting arguments for that
// message, where N is the number of % symbols in msg. Following that, another
// string may be added to become the next error in the chain. Each new error
// will be wrapped by its prior error.
//
//	 err := errf(
//	   "could not process %v", thing,
//	   "name %q is invalid", thing.Name,
//	)
//	fmt.Println(err)  // could not process Thing: name Foo is invalid
//	fmt.Println(RootCause(err))  // name Foo is invalid
//
// In place of a string, the last error can be another error, in which case it
// will be treated as the cause of the prior error chain.
//
//	 errf(
//	   "could not process %v", thing,
//	   "date %q could not be parsed", thing.Date,
//	   parseError,
//	)
func errf(msg string, args ...interface{}) error {
	// By implementing buildErrf as a closure rather than a standalone
	// function, we're able to ensure that it is called only from errf, or
	// from itself (recursively). By controlling these invocations in such
	// a tight space, we are able to easily verify manually that we
	// checked len(args) > 0 before making the call.
	var buildErrf func([]interface{}) error
	buildErrf = func(args []interface{}) error {
		arg, args := args[0], args[1:] // assume len(args) > 0
		if arg == nil {
			digerror.BugPanicf("arg must not be nil")
		}

		switch v := arg.(type) {
		case string:
			need := numFmtArgs(v)
			if len(args) < need {
				digerror.BugPanicf("string %q needs %v arguments, got %v",
					v,
					need,
					len(args))
			}

			msg := fmt.Sprintf(v, args[:need]...)
			args := args[need:]

			// If we don't have anything left to chain with, build the
			// final error.
			if len(args) == 0 {
				return newError(msg)
			}

			return defaultWrappedError{
				msg: msg,
				err: buildErrf(args),
			}
		case error:
			if len(args) > 0 {
				digerror.BugPanicf("error must be the last element but got %v", args)
			}
			return v
		default:
			digerror.BugPanicf("unexpected errf-argument type %T", arg)
			return nil
		}
	}

	// Prepend msg to the args list so that we can re-use the same
	// args processing logic. The msg is a string just for type-safety of
	// the first error.
	newArgs := make([]interface{}, len(args)+1)
	newArgs[0] = msg
	copy(newArgs[1:], args)
	return buildErrf(newArgs)
}

// Returns the number of formatting arguments in the provided string. Does not
// count escaped % symbols, specifically the string "%%".
//
//	fmt.Println(numFmtArgs("rate: %d%%"))  // 1
func numFmtArgs(s string) int {
	var (
		count   int
		percent bool // saw %
	)
	for _, c := range s {
		if percent && c != '%' {
			// Counts only if it's not a %%.
			count++
		}

		// Next iteration should consider % only if the current %
		// stands alone.
		percent = !percent && c == '%'
	}
	return count
}

// defaultWrappedError is returned by errf when a chain of errors needs to be created
type defaultWrappedError struct {
	err error
	msg string
}

var _ DigError = defaultWrappedError{}

func (e defaultWrappedError) dummy() {}

func (e defaultWrappedError) Unwrap() error {
	return e.err
}

func (e defaultWrappedError) writeMessage(w io.Writer, _ string) {
	io.WriteString(w, e.msg)
}

func (e defaultWrappedError) Error() string { return fmt.Sprint(e) }
func (e defaultWrappedError) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// errProvide is returned when a constructor could not be Provided into the
// container.
type errProvide struct {
	Func   *digreflect.Func
	Reason error
}

var _ DigError = errProvide{}

func (e errProvide) dummy() {}

func (e errProvide) Unwrap() error {
	return e.Reason
}

func (e errProvide) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "cannot provide function "+verb, e.Func)
}

func (e errProvide) Error() string { return fmt.Sprint(e) }
func (e errProvide) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// errConstructorFailed is returned when a user-provided constructor failed
// with a non-nil error.
type errConstructorFailed struct {
	Func   *digreflect.Func
	Reason error
}

var _ DigError = errConstructorFailed{}

func (e errConstructorFailed) dummy() {}

func (e errConstructorFailed) Unwrap() error {
	return e.Reason
}

func (e errConstructorFailed) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "received non-nil error from function "+verb, e.Func)
}

func (e errConstructorFailed) Error() string { return fmt.Sprint(e) }
func (e errConstructorFailed) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// errValueGroup is a more specific type of specification error specific to value groups
type errValueGroup struct {
	Message string
}

var _ DigError = errValueGroup{}

func (e errValueGroup) dummy() {}

func (e errValueGroup) Error() string { return e.Message }

// errArgumentsFailed is returned when a function could not be run because one
// of its dependencies failed to build for any reason.
type errArgumentsFailed struct {
	Func   *digreflect.Func
	Reason error
}

var _ DigError = errArgumentsFailed{}

func (e errArgumentsFailed) dummy() {}

func (e errArgumentsFailed) Unwrap() error {
	return e.Reason
}

func (e errArgumentsFailed) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "could not build arguments for function "+verb, e.Func)
}

func (e errArgumentsFailed) Error() string { return fmt.Sprint(e) }
func (e errArgumentsFailed) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// errMissingDependencies is returned when the dependencies of a function are
// not available in the container.
type errMissingDependencies struct {
	Func   *digreflect.Func
	Reason error
}

var _ DigError = errMissingDependencies{}

func (e errMissingDependencies) dummy() {}

func (e errMissingDependencies) Unwrap() error {
	return e.Reason
}

func (e errMissingDependencies) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "missing dependencies for function "+verb, e.Func)
}

func (e errMissingDependencies) Error() string { return fmt.Sprint(e) }
func (e errMissingDependencies) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

// errParamSingleFailed is returned when a paramSingle could not be built.
type errParamSingleFailed struct {
	Key    key
	Reason error
	CtorID dot.CtorID
}

var _ DigError = errParamSingleFailed{}

func (e errParamSingleFailed) dummy() {}

func (e errParamSingleFailed) Unwrap() error {
	return e.Reason
}

func (e errParamSingleFailed) writeMessage(w io.Writer, _ string) {
	fmt.Fprintf(w, "failed to build %v", e.Key)
}

func (e errParamSingleFailed) Error() string { return fmt.Sprint(e) }
func (e errParamSingleFailed) Format(w fmt.State, c rune) {
	formatError(e, w, c)
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

var _ DigError = errParamGroupFailed{}

func (e errParamGroupFailed) dummy() {}

func (e errParamGroupFailed) Unwrap() error {
	return e.Reason
}

func (e errParamGroupFailed) writeMessage(w io.Writer, _ string) {
	fmt.Fprintf(w, "could not build value group %v", e.Key)
}

func (e errParamGroupFailed) Error() string { return fmt.Sprint(e) }
func (e errParamGroupFailed) Format(w fmt.State, c rune) {
	formatError(e, w, c)
}

func (e errParamGroupFailed) updateGraph(g *dot.Graph) {
	g.FailGroupNodes(e.Key.group, e.Key.t, e.CtorID)
}

// missingType holds information about a type that was missing in the
// container.
type missingType struct {
	Key key // item that was missing

	// If non-empty, we will include suggestions for what the user may have
	// meant.
	suggestions []key
}

// Format prints a string representation of missingType.
//
// With %v, it prints a short representation ideal for an itemized list.
//
//	io.Writer
//	io.Writer: did you mean *bytes.Buffer?
//	io.Writer: did you mean *bytes.Buffer, or *os.File?
//
// With %+v, it prints a longer representation ideal for standalone output.
//
//	io.Writer: did you mean to Provide it?
//	io.Writer: did you mean to use *bytes.Buffer?
//	io.Writer: did you mean to use one of *bytes.Buffer, or *os.File?
func (mt missingType) Format(w fmt.State, v rune) {
	plusV := w.Flag('+') && v == 'v'

	fmt.Fprint(w, mt.Key)
	switch len(mt.suggestions) {
	case 0:
		if plusV {
			io.WriteString(w, " (did you mean to Provide it?)")
		}
	case 1:
		sug := mt.suggestions[0]
		if plusV {
			fmt.Fprintf(w, " (did you mean to use %v?)", sug)
		} else {
			fmt.Fprintf(w, " (did you mean %v?)", sug)
		}
	default:
		if plusV {
			io.WriteString(w, " (did you mean to use one of ")
		} else {
			io.WriteString(w, " (did you mean ")
		}

		lastIdx := len(mt.suggestions) - 1
		for i, sug := range mt.suggestions {
			if i > 0 {
				io.WriteString(w, ", ")
				if i == lastIdx {
					io.WriteString(w, "or ")
				}
			}
			fmt.Fprint(w, sug)
		}
		io.WriteString(w, "?)")
	}
}

// errMissingType is returned when one or more values that were expected in
// the container were not available.
//
// Multiple instances of this error may be merged together by appending them.
type errMissingTypes []missingType // inv: len > 0

var _ DigError = make(errMissingTypes, 0)

func newErrMissingTypes(c containerStore, k key) errMissingTypes {
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

	mt := missingType{Key: k}
	for _, t := range suggestions {
		if len(c.getValueProviders(k.name, t)) > 0 {
			k.t = t
			mt.suggestions = append(mt.suggestions, k)
		}
	}

	return errMissingTypes{mt}
}

func (e errMissingTypes) dummy() {}

func (e errMissingTypes) Error() string {
	return fmt.Sprint(e)
}

func (e errMissingTypes) Format(w fmt.State, v rune) {
	multiline := w.Flag('+') && v == 'v'

	if len(e) == 1 {
		io.WriteString(w, "missing type:")
	} else {
		io.WriteString(w, "missing types:")
	}

	if !multiline {
		// With %v, we need a space between : since the error
		// won't be on a new line.
		io.WriteString(w, " ")
	}

	for i, mt := range e {
		if multiline {
			io.WriteString(w, "\n\t- ")
		} else if i > 0 {
			io.WriteString(w, "; ")
		}

		if multiline {
			fmt.Fprintf(w, "%+v", mt)
		} else {
			fmt.Fprintf(w, "%v", mt)
		}
	}
}

func (e errMissingTypes) updateGraph(g *dot.Graph) {
	missing := make([]*dot.Result, len(e))

	for i, mt := range e {
		missing[i] = &dot.Result{
			Node: &dot.Node{
				Name:  mt.Key.name,
				Group: mt.Key.group,
				Type:  mt.Key.t,
			},
		}
	}
	g.AddMissingNodes(missing)
}

type errVisualizer interface {
	updateGraph(*dot.Graph)
}

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
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"

	"go.uber.org/dig/internal/digreflect"
	"go.uber.org/dig/internal/dot"
)

// Errors which know their underlying cause should implement this interface to
// be compatible with RootCause.
//
// We use unexported methods because we don't want dig-internal causes to be
// confused with the cause of the user-provided errors. For example, if we
// used Unwrap(), then user-provided methods would also be unwrapped by
// RootCause. We want RootCause to eliminate the dig error chain only.
type causer interface {
	fmt.Formatter

	// Returns the next error in the chain.
	cause() error

	// Writes the message or context for this error in the chain.
	//
	// verb is either %v or %+v.
	writeMessage(w io.Writer, verb string)
}

// Implements fmt.Formatter for errors that implement causer.
//
// This Format method supports %v and %+v. In the %v form, the error is
// printed on one line. In the %+v form, the error is split across multiple
// lines on each error in the error chain.
func formatCauser(c causer, w fmt.State, v rune) {
	multiline := w.Flag('+') && v == 'v'
	verb := "%v"
	if multiline {
		verb = "%+v"
	}

	// "context: " or "context:\n"
	c.writeMessage(w, verb)
	io.WriteString(w, ":")
	if multiline {
		io.WriteString(w, "\n")
	} else {
		io.WriteString(w, " ")
	}

	fmt.Fprintf(w, verb, c.cause())
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

// errf is a version of fmt.Errorf with support for a chain of multiple
// formatted error messages.
//
// After msg, N arguments are consumed as formatting arguments for that
// message, where N is the number of % symbols in msg. Following that, another
// string may be added to become the next error in the chain. Each new error
// is the `cause()` for the prior error.
//
//   err := errf(
//     "could not process %v", thing,
//     "name %q is invalid", thing.Name,
//  )
//  fmt.Println(err)  // could not process Thing: name Foo is invalid
//  fmt.Println(RootCause(err))  // name Foo is invalid
//
// In place of a string, the last error can be another error, in which case it
// will be treated as the cause of the prior error chain.
//
//   errf(
//     "could not process %v", thing,
//     "date %q could not be parsed", thing.Date,
//     parseError,
//  )
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
			panic("It looks like you have found a bug in dig. " +
				"Please file an issue at https://github.com/uber-go/dig/issues/ " +
				"and provide the following message: " +
				"arg must not be nil")
		}

		switch v := arg.(type) {
		case string:
			need := numFmtArgs(v)
			if len(args) < need {
				panic(fmt.Sprintf(
					"It looks like you have found a bug in dig. "+
						"Please file an issue at https://github.com/uber-go/dig/issues/ "+
						"and provide the following message: "+
						"string %q needs %v arguments, got %v", v, need, len(args)))
			}

			msg := fmt.Sprintf(v, args[:need]...)
			args := args[need:]

			// If we don't have anything left to chain with, build the
			// final error.
			if len(args) == 0 {
				return errors.New(msg)
			}

			return wrappedError{
				msg: msg,
				err: buildErrf(args),
			}
		case error:
			if len(args) > 0 {
				panic(fmt.Sprintf(
					"It looks like you have found a bug in dig. "+
						"Please file an issue at https://github.com/uber-go/dig/issues/ "+
						"and provide the following message: "+
						"error must be the last element but got %v", args))
			}

			return v

		default:
			panic(fmt.Sprintf(
				"It looks like you have found a bug in dig. "+
					"Please file an issue at https://github.com/uber-go/dig/issues/ "+
					"and provide the following message: "+
					"unexpected errf-argument type %T", arg))
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
//   fmt.Println(numFmtArgs("rate: %d%%"))  // 1
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

type wrappedError struct {
	err error
	msg string
}

var _ causer = wrappedError{}

func (e wrappedError) cause() error {
	return e.err
}

func (e wrappedError) writeMessage(w io.Writer, _ string) {
	io.WriteString(w, e.msg)
}

func (e wrappedError) Error() string { return fmt.Sprint(e) }
func (e wrappedError) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

// errProvide is returned when a constructor could not be Provided into the
// container.
type errProvide struct {
	Func   *digreflect.Func
	Reason error
}

var _ causer = errProvide{}

func (e errProvide) cause() error {
	return e.Reason
}

func (e errProvide) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "cannot provide function "+verb, e.Func)
}

func (e errProvide) Error() string { return fmt.Sprint(e) }
func (e errProvide) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

// errConstructorFailed is returned when a user-provided constructor failed
// with a non-nil error.
type errConstructorFailed struct {
	Func   *digreflect.Func
	Reason error
}

var _ causer = errConstructorFailed{}

func (e errConstructorFailed) cause() error {
	return e.Reason
}

func (e errConstructorFailed) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "received non-nil error from function "+verb, e.Func)
}

func (e errConstructorFailed) Error() string { return fmt.Sprint(e) }
func (e errConstructorFailed) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

// errArgumentsFailed is returned when a function could not be run because one
// of its dependencies failed to build for any reason.
type errArgumentsFailed struct {
	Func   *digreflect.Func
	Reason error
}

var _ causer = errArgumentsFailed{}

func (e errArgumentsFailed) cause() error {
	return e.Reason
}

func (e errArgumentsFailed) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "could not build arguments for function "+verb, e.Func)
}

func (e errArgumentsFailed) Error() string { return fmt.Sprint(e) }
func (e errArgumentsFailed) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

// errMissingDependencies is returned when the dependencies of a function are
// not available in the container.
type errMissingDependencies struct {
	Func   *digreflect.Func
	Reason error
}

var _ causer = errMissingDependencies{}

func (e errMissingDependencies) cause() error {
	return e.Reason
}

func (e errMissingDependencies) writeMessage(w io.Writer, verb string) {
	fmt.Fprintf(w, "missing dependencies for function "+verb, e.Func)
}

func (e errMissingDependencies) Error() string { return fmt.Sprint(e) }
func (e errMissingDependencies) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
}

// errParamSingleFailed is returned when a paramSingle could not be built.
type errParamSingleFailed struct {
	Key    key
	Reason error
	CtorID dot.CtorID
}

var _ causer = errParamSingleFailed{}

func (e errParamSingleFailed) cause() error {
	return e.Reason
}

func (e errParamSingleFailed) writeMessage(w io.Writer, _ string) {
	fmt.Fprintf(w, "failed to build %v", e.Key)
}

func (e errParamSingleFailed) Error() string { return fmt.Sprint(e) }
func (e errParamSingleFailed) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
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

var _ causer = errParamGroupFailed{}

func (e errParamGroupFailed) cause() error {
	return e.Reason
}

func (e errParamGroupFailed) writeMessage(w io.Writer, _ string) {
	fmt.Fprintf(w, "could not build value group %v", e.Key)
}

func (e errParamGroupFailed) Error() string { return fmt.Sprint(e) }
func (e errParamGroupFailed) Format(w fmt.State, c rune) {
	formatCauser(e, w, c)
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

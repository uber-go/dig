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
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig/internal/digreflect"
)

// assertErrorMatches matches error messages against the provided list of
// strings.
//
// The error must match each string in-order. That is, the following is valid,
//
//   assertErrorMatches(t, errors.New("foo bar baz"), "foo", "baz")
//
// But not,
//
//   assertErrorMatches(t, errors.New("foo bar baz"), "foo", "baz", "bar")
//
// Because "bar" is not after "baz" in the error message.
//
// Messages will be treated as regular expressions.
func assertErrorMatches(t *testing.T, err error, msg string, msgs ...string) {
	// We have one positional argument in addition to the variadic argument to
	// ensure that there's at least one string to match against.
	if err == nil {
		t.Errorf("expected error but got nil")
		return
	}

	var finders []consumingFinder
	for _, m := range append([]string{msg}, msgs...) {
		if r, err := regexp.Compile(m); err == nil {
			finders = append(finders, regexpFinder{r})
		} else {
			finders = append(finders, stringFinder(m))
		}
	}

	t.Run("single line", func(t *testing.T) {
		original := err.Error()
		assert.NoError(t, runFinders(original, finders))
	})

	// Intersperse "\n" finders between each message for the "%+v" check.
	plusFinders := make([]consumingFinder, 0, len(finders)*2-1)
	for i, f := range finders {
		if i > 0 {
			plusFinders = append(plusFinders, stringFinder("\n"))
		}
		plusFinders = append(plusFinders, f)
	}

	t.Run("multi line", func(t *testing.T) {
		original := fmt.Sprintf("%+v", err)
		assert.NoError(t, runFinders(original, plusFinders))
	})
}

// consumingFinder matches a string and returns the rest of the string *after*
// the match.
type consumingFinder interface {
	// Attempt to match against the given string and return false if a match
	// could not be found.
	//
	// If a match was found, return the remaining string after the entire
	// match. So if the finder matches "oo" in "foobar", the returned string
	// must be just "bar".
	Find(got string) (rest string, ok bool)
}

func runFinders(original string, finders []consumingFinder) error {
	remaining := original
	for _, f := range finders {
		if newRemaining, ok := f.Find(remaining); ok {
			remaining = newRemaining
			continue
		}

		// Match not found. Check if the order was wrong.
		if _, ok := f.Find(original); ok {
			// We won't use %q for the error message itself
			// because we want it to be printed to the console as
			// it would actually show.
			return errf(`"%v" contains %v in the wrong place`, original, f)
		}
		return errf(`"%v" does not contain %v`, original, f)
	}
	return nil
}

type regexpFinder struct{ r *regexp.Regexp }

func (r regexpFinder) String() string {
	return "`" + r.r.String() + "`"
}

func (r regexpFinder) Find(got string) (rest string, ok bool) {
	loc := r.r.FindStringIndex(got)
	if len(loc) == 0 {
		return got, false
	}
	return got[loc[1]:], true
}

type stringFinder string

func (s stringFinder) String() string { return strconv.Quote(string(s)) }

func (s stringFinder) Find(got string) (rest string, ok bool) {
	i := strings.Index(got, string(s))
	if i < 0 {
		return got, false
	}
	return got[i+len(s):], true
}

func TestErrf(t *testing.T) {
	type args = []interface{}

	tests := []struct {
		desc string
		give error

		wantV     string // output for %v
		wantPlusV string // output for %+v

		wantRootCause error
	}{
		{
			desc:          "single unformatted error",
			give:          errf("foo"),
			wantV:         "foo",
			wantPlusV:     "foo",
			wantRootCause: errors.New("foo"),
		},
		{
			desc:          "single formatted error",
			give:          errf("foo %d %s", 42, "bar"),
			wantV:         "foo 42 bar",
			wantPlusV:     "foo 42 bar",
			wantRootCause: errors.New("foo 42 bar"),
		},
		{
			desc:          "multiple unformatted errors",
			give:          errf("foo", "bar", "baz"),
			wantV:         "foo: bar: baz",
			wantPlusV:     joinLines("foo:", "bar:", "baz"),
			wantRootCause: errors.New("baz"),
		},
		{
			desc:          "multiple formatted errors",
			give:          errf("foo %d", 42, "bar %s", "baz", "qux %q", "quux"),
			wantV:         `foo 42: bar baz: qux "quux"`,
			wantPlusV:     joinLines("foo 42:", "bar baz:", `qux "quux"`),
			wantRootCause: errors.New(`qux "quux"`),
		},
		{
			desc:          "single error",
			give:          errf("foo", "bar", errors.New("great sadness")),
			wantV:         "foo: bar: great sadness",
			wantPlusV:     joinLines("foo:", "bar:", "great sadness"),
			wantRootCause: errors.New("great sadness"),
		},
		{
			desc:          "multiple errors",
			give:          errf("foo", "bar: %v", errors.New("baz"), errors.New("great sadness")),
			wantV:         "foo: bar: baz: great sadness",
			wantPlusV:     joinLines("foo:", "bar: baz:", "great sadness"),
			wantRootCause: errors.New("great sadness"),
		},
		{
			desc:          "escaped percent",
			give:          errf("foo %% %v", "bar"),
			wantV:         "foo % bar",
			wantPlusV:     "foo % bar",
			wantRootCause: errors.New("foo % bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.give
			require.NotNil(t, err, "invalid test: must not be nil")

			t.Run("Error", func(t *testing.T) {
				assert.Equal(t, tt.wantV, err.Error())
			})

			t.Run("format with %+v", func(t *testing.T) {
				assert.Equal(t, tt.wantPlusV, fmt.Sprintf("%+v", err))
			})

			t.Run("RootCause", func(t *testing.T) {
				assert.Equal(t, tt.wantRootCause, RootCause(err))
			})
		})
	}
}

func TestErrfInvalid(t *testing.T) {
	t.Run("nil panics", func(t *testing.T) {
		assert.Panics(t, func() {
			errf("foo", nil)
		})
	})

	t.Run("too few argumetns", func(t *testing.T) {
		assert.Panics(t, func() {
			errf("foo %v")
		})
	})

	t.Run("error before last", func(t *testing.T) {
		assert.Panics(t, func() {
			errf("foo", errors.New("bar"), "baz %v", 42)
		})
	})

	t.Run("unknown type", func(t *testing.T) {
		assert.Panics(t, func() {
			errf("foo %v", 42, 43)
		})
	})
}

func TestNumFmtArgs(t *testing.T) {
	tests := []struct {
		desc string
		give string
		want int
	}{
		{"empty", "", 0},
		{"none", "foo bar", 0},
		{"some", "foo %v b %d ar", 2},
		{"trailing", "foo %v 100%", 1},
		{"escaped", "foo %v bar %% baz %d", 2},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.want, numFmtArgs(tt.give))
		})
	}
}

func joinLines(ls ...string) string { return strings.Join(ls, "\n") }

// Simple error fake that provides control of %v and %+v representations.
type errFormatted struct {
	v     string //  output for %v
	plusV string // output for %+v
}

var (
	_ error         = errFormatted{}
	_ fmt.Formatter = errFormatted{}
)

func (e errFormatted) Error() string { return e.v }

func (e errFormatted) Format(w fmt.State, c rune) {
	if w.Flag('+') && c == 'v' {
		io.WriteString(w, e.plusV)
	} else {
		io.WriteString(w, e.v)
	}
}

func TestMissingTypeFormatting(t *testing.T) {
	type type1 struct{}
	type someInterface interface{ stuff() }

	tests := []struct {
		desc      string
		give      missingType
		wantV     string
		wantPlusV string
	}{
		{
			desc: "no suggestions",
			give: missingType{
				Key: key{t: reflect.TypeOf(type1{})},
			},
			wantV:     "dig.type1",
			wantPlusV: "dig.type1 (did you mean to Provide it?)",
		},
		{
			desc: "one suggestion",
			give: missingType{
				Key: key{t: reflect.TypeOf(type1{})},
				suggestions: []key{
					{t: reflect.TypeOf(&type1{})},
				},
			},
			wantV:     "dig.type1 (did you mean *dig.type1?)",
			wantPlusV: "dig.type1 (did you mean to use *dig.type1?)",
		},
		{
			desc: "many suggestions",
			give: missingType{
				Key: key{t: reflect.TypeOf(type1{})},
				suggestions: []key{
					{t: reflect.TypeOf(&type1{})},
					{t: reflect.TypeOf(new(someInterface)).Elem()},
				},
			},
			wantV:     "dig.type1 (did you mean *dig.type1, or dig.someInterface?)",
			wantPlusV: "dig.type1 (did you mean to use one of *dig.type1, or dig.someInterface?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.wantV, fmt.Sprint(tt.give), "%v did not match")
			assert.Equal(t, tt.wantPlusV, fmt.Sprintf("%+v", tt.give), "%+v did not match")
		})
	}
}

func TestErrorFormatting(t *testing.T) {
	type someType struct{}
	type anotherType struct{}

	simpleErr := errors.New("great sadness")
	richError := errFormatted{
		v: "great sadness",
		plusV: joinLines(
			"sadness so great",
			"it needs multiple",
			"lines",
		),
	}

	someFunc := &digreflect.Func{
		Package: "foo",
		Name:    "Bar",
		File:    "foo/bar.go",
		Line:    42,
	}

	tests := []struct {
		desc       string
		give       error
		wantString string
		wantPlusV  string
	}{
		{
			desc: "wrappedError/simple",
			give: wrappedError{
				msg: "something went wrong",
				err: simpleErr,
			},
			wantString: "something went wrong: great sadness",
			wantPlusV: joinLines(
				"something went wrong:",
				"great sadness",
			),
		},
		{
			desc: "wrappedError/rich",
			give: wrappedError{
				msg: "something went wrong",
				err: richError,
			},
			wantString: "something went wrong: great sadness",
			wantPlusV: joinLines(
				"something went wrong:",
				"sadness so great",
				"it needs multiple",
				"lines",
			),
		},
		{
			desc: "errProvide",
			give: errProvide{
				Func:   someFunc,
				Reason: simpleErr,
			},
			wantString: `cannot provide function "foo".Bar (foo/bar.go:42): great sadness`,
			wantPlusV: joinLines(
				`cannot provide function "foo".Bar`,
				"	foo/bar.go:42:",
				"great sadness",
			),
		},
		{
			desc: "errConstructorFailed",
			give: errConstructorFailed{
				Func:   someFunc,
				Reason: richError,
			},
			wantString: `received non-nil error from function "foo".Bar (foo/bar.go:42): great sadness`,
			wantPlusV: joinLines(
				`received non-nil error from function "foo".Bar`,
				"	foo/bar.go:42:",
				"sadness so great",
				"it needs multiple",
				"lines",
			),
		},
		{
			desc: "errArgumentsFailed",
			give: errArgumentsFailed{
				Func:   someFunc,
				Reason: simpleErr,
			},
			wantString: `could not build arguments for function "foo".Bar (foo/bar.go:42): great sadness`,
			wantPlusV: joinLines(
				`could not build arguments for function "foo".Bar`,
				"	foo/bar.go:42:",
				"great sadness",
			),
		},
		{
			desc: "errMissingDependencies",
			give: errMissingDependencies{
				Func:   someFunc,
				Reason: richError,
			},
			wantString: `missing dependencies for function "foo".Bar (foo/bar.go:42): great sadness`,
			wantPlusV: joinLines(
				`missing dependencies for function "foo".Bar`,
				"	foo/bar.go:42:",
				"sadness so great",
				"it needs multiple",
				"lines",
			),
		},
		{
			desc: "errParamSingleFailed",
			give: errParamSingleFailed{
				Key:    key{t: reflect.TypeOf(someType{})},
				Reason: richError,
			},
			wantString: `failed to build dig.someType: great sadness`,
			wantPlusV: joinLines(
				`failed to build dig.someType:`,
				"sadness so great",
				"it needs multiple",
				"lines",
			),
		},
		{
			desc: "errParamGroupFailed",
			give: errParamGroupFailed{
				Key:    key{t: reflect.TypeOf(someType{}), group: "items"},
				Reason: richError,
			},
			wantString: `could not build value group dig.someType[group="items"]: great sadness`,
			wantPlusV: joinLines(
				`could not build value group dig.someType[group="items"]:`,
				"sadness so great",
				"it needs multiple",
				"lines",
			),
		},
		{
			desc: "errMissingTypes/single",
			give: errMissingTypes{
				{Key: key{t: reflect.TypeOf(someType{})}},
			},
			wantString: "missing type: dig.someType",
			wantPlusV: joinLines(
				"missing type:",
				"	- dig.someType (did you mean to Provide it?)",
			),
		},
		{
			desc: "errMissingTypes/multiple",
			give: errMissingTypes{
				{Key: key{t: reflect.TypeOf(someType{})}},
				{Key: key{t: reflect.TypeOf(&anotherType{})}},
			},
			wantString: "missing types: dig.someType; *dig.anotherType",
			wantPlusV: joinLines(
				"missing types:",
				"	- dig.someType (did you mean to Provide it?)",
				"	- *dig.anotherType (did you mean to Provide it?)",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.wantString, tt.give.Error(), "%v did not match")
			assert.Equal(t, tt.wantPlusV, fmt.Sprintf("%+v", tt.give), "%+v did not match")
		})
	}
}

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
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrWrapf(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		err := errWrapf(nil, "hi")
		assert.NoError(t, err, "expected no error")
		assert.NoError(t, RootCause(err), "root cause must be nil")
	})

	t.Run("single wrap", func(t *testing.T) {
		err := errors.New("great sadness")
		werr := errWrapf(err, "something went %s", "wrong")

		assert.Equal(t, err, RootCause(werr), "root cause must match")
		assert.Equal(t, "something went wrong: great sadness", werr.Error(),
			"error message must match")
	})

	t.Run("double wrap", func(t *testing.T) {
		err := errors.New("great sadness")

		werr := errWrapf(err, "something went %s", "wrong")
		werr = errWrapf(werr, "something else went wrong")

		assert.Equal(t, err, RootCause(werr), "root cause must match")
		assert.Equal(t, "something else went wrong: something went wrong: great sadness", werr.Error(),
			"error message must match")
	})
}

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
func assertErrorMatches(t testing.TB, err error, msg string, msgs ...string) {
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

	original := err.Error()
	remaining := original
	for _, f := range finders {
		if newRemaining, ok := f.Find(remaining); ok {
			remaining = newRemaining
			continue
		}

		// Match not found. Check if the order was wrong.
		if _, ok := f.Find(original); ok {
			// We won't use %q for the error message itself because we want it
			// to be printed to the console as it would actually show.
			t.Errorf(`"%v" contains %v in the wrong place`, original, f)
		} else {
			t.Errorf(`"%v" does not contain %v`, original, f)
		}
	}
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

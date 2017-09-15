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
	"errors"
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

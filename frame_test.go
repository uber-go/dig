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
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomFrameSkipper(t *testing.T) {
	s := func(f runtime.Frame) bool {
		// do not skip any frames
		return false
	}
	c := New(WithFrameSkipper(s))

	type A struct{}

	var got string
	err := c.Provide(
		func() A { return A{} },
		WithProvideHook(func(e ProvideEvent) {
			got = e.Caller
		}),
	)
	require.NoError(t, err)
	assert.Contains(t, got, "dig.(*Container).Provide")
}

func TestCaller(t *testing.T) {
	t.Run("skipping all frames", func(t *testing.T) {
		c := getCaller(func(f runtime.Frame) bool {
			return true
		})
		assert.Contains(t, c, "n/a")
	})

	t.Run("default skipper", func(t *testing.T) {
		c := getCaller(defaultFrameSkipper)
		assert.Contains(t, c, "dig.TestCaller")
	})
}

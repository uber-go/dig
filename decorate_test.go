// Copyright (c) 2021 Uber Technologies, Inc.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecorateSuccess(t *testing.T) {
	t.Run("simple decorate without names or groups", func(t *testing.T) {
		c := New()

		type A struct {
			name string
		}

		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, a.name, "A", "expected name to not be decorated yet.")
		}))

		require.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, a.name, "A'", "expected name to equal decorated name.")
		}))
	})

	t.Run("simple decorate a provider from child scope", func(t *testing.T) {
		c := New()
		type A struct {
			name string
		}

		child := c.Scope("child")
		require.NoError(t, child.Provide(func() *A { return &A{name: "A"} }, Export(true)))

		c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} })

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, a.name, "A'", "expected name to equal decorated name in parent scope")
		}))

		assert.NoError(t, child.Invoke(func(a *A) {
			assert.Equal(t, a.name, "A'", "expected name to equal original name in child scope")
		}))
	})
}

func TestDecorateFailure(t *testing.T) {
	t.Run("decorate a type that wasn't provided", func(t *testing.T) {
		c := New()

		type A struct {
			name string
		}

		err := c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} })

		assert.Error(t, err)

		assert.Contains(t, err.Error(), "*dig.A was never Provided to Scope [container]")
	})

	t.Run("decorate the same type twice", func(t *testing.T) {
		c := New()
		type A struct {
			name string
		}
		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))

		assert.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }), "first decorate should not fail.")
		err := c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} })
		assert.Error(t, err, "expected second call to decorate to fail.")
		assert.Contains(t, err.Error(), "*dig.A was already Decorated in Scope [container]")
	})
}

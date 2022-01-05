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
		t.Parallel()
		type A struct {
			name string
		}

		c := New()
		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, "A", a.name, "expected name to not be decorated yet.")
		}))

		require.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected name to equal decorated name.")
		}))
	})

	t.Run("simple decorate a provider from child scope", func(t *testing.T) {
		t.Parallel()
		type A struct {
			name string
		}

		c := New()
		child := c.Scope("child")
		require.NoError(t, child.Provide(func() *A { return &A{name: "A"} }, Export(true)))

		assert.NoError(t, child.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))
		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, "A", a.name, "expected name to equal original name in parent scope")
		}))
		assert.NoError(t, child.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected name to equal decorated name in child scope")
		}))
	})

	t.Run("simple decorate a provider to a scope and its descendants", func(t *testing.T) {
		t.Parallel()
		type A struct {
			name string
		}

		c := New()
		child := c.Scope("child")
		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))

		assert.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))
		assertDecoratedName := func(a *A) {
			assert.Equal(t, a.name, "A'", "expected name to equal decorated name")
		}
		assert.NoError(t, c.Invoke(assertDecoratedName))
		assert.NoError(t, child.Invoke(assertDecoratedName))
	})

	t.Run("modifications compose with descendants", func(t *testing.T) {
		t.Parallel()
		type A struct {
			name string
		}

		c := New()
		child := c.Scope("child")
		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))

		require.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))
		require.NoError(t, child.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))

		assert.NoError(t, c.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected decorated name in parent")
		}))

		assert.NoError(t, child.Invoke(func(a *A) {
			assert.Equal(t, "A''", a.name, "expected double-decorated name in child")
		}))
	})

	t.Run("decorate with In struct", func(t *testing.T) {
		t.Parallel()

		type A struct {
			Name string
		}
		type B struct {
			In

			A *A
			B string `name:"b"`
		}

		type C struct {
			Out

			A *A
			B string `name:"b"`
		}

		c := New()
		require.NoError(t, c.Provide(func() *A { return &A{Name: "A"} }))
		require.NoError(t, c.Provide(func() string { return "b" }, Name("b")))

		require.NoError(t, c.Decorate(func(b B) C {
			return C{
				A: &A{
					Name: b.A.Name + "'",
				},
				B: b.B + "'",
			}
		}))

		assert.NoError(t, c.Invoke(func(b B) {
			assert.Equal(t, "A'", b.A.Name)
			assert.Equal(t, "b'", b.B)
		}))
	})

	t.Run("decorate with value groups", func(t *testing.T) {
		type Params struct {
			In

			Animals []string `group:"animals"`
		}

		type Result struct {
			Out

			Animals []string `group:"animals"`
		}

		c := New()
		require.NoError(t, c.Provide(func() string { return "dog" }, Group("animals")))
		require.NoError(t, c.Provide(func() string { return "cat" }, Group("animals")))
		require.NoError(t, c.Provide(func() string { return "alpaca" }, Group("animals")))

		assert.NoError(t, c.Decorate(func(p Params) Result {
			animals := p.Animals
			for i := 0; i < len(animals); i++ {
				animals[i] = "good " + animals[i]
			}
			return Result{
				Animals: animals,
			}
		}))

		assert.NoError(t, c.Invoke(func(p Params) {
			assert.Contains(t, p.Animals, "good dog")
		}))
	})
}

func TestDecorateFailure(t *testing.T) {
	t.Run("decorate a type that wasn't provided", func(t *testing.T) {
		c := New()

		type A struct {
			name string
		}

		require.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }))
		err := c.Invoke(func(a *A) string { return a.name })

		assert.Error(t, err)

		assert.Contains(t, err.Error(), "missing type: *dig.A")
	})

	t.Run("decorate the same type twice", func(t *testing.T) {
		c := New()
		type A struct {
			name string
		}
		require.NoError(t, c.Provide(func() *A { return &A{name: "A"} }))
		require.NoError(t, c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} }), "first decorate should not fail.")

		err := c.Decorate(func(a *A) *A { return &A{name: a.name + "'"} })
		require.Error(t, err, "expected second call to decorate to fail.")
		assert.Contains(t, err.Error(), "*dig.A was already Decorated")
	})
}

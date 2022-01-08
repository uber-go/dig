// Copyright (c) 2022 Uber Technologies, Inc.
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

		sibling := c.Scope("sibling")
		grandchild := child.Scope("grandchild")
		sibling.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected single-decorated name in sibling")
		})
		grandchild.Invoke(func(a *A) {
			assert.Equal(t, "A''", a.name, "expected double-decorated name in grandchild")
		})
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

	t.Run("decorate with optional parameter", func(t *testing.T) {
		c := New()

		type A struct{}
		type Param struct {
			In

			Values []string `group:"values"`
			A      *A       `optional:"true"`
		}

		type Result struct {
			Out

			Values []string `group:"values"`
		}

		require.NoError(t, c.Provide(func() string { return "a" }, Group("values")))
		require.NoError(t, c.Provide(func() string { return "b" }, Group("values")))

		require.NoError(t, c.Decorate(func(p Param) Result {
			return Result{
				Values: append(p.Values, "c"),
			}
		}))

		assert.NoError(t, c.Invoke(func(p Param) {
			assert.Equal(t, 3, len(p.Values))
		}))
	})

	t.Run("replace a type completely", func(t *testing.T) {
		t.Parallel()

		c := New()
		type A struct {
			From string
		}

		require.NoError(t, c.Provide(func() A {
			assert.Fail(t, "provider shouldn't be called")
			return A{From: "provider"}
		}))
		require.NoError(t, c.Decorate(func() A {
			return A{From: "decorator"}
		}))
		require.NoError(t, c.Invoke(func(a A) {
			assert.Equal(t, a.From, "decorator", "value should be from decorator")
		}))
	})

	t.Run("group value decorator from parent and child", func(t *testing.T) {
		t.Parallel()

		type DecorateIn struct {
			In

			Values []string `group:"values"`
		}

		type DecorateOut struct {
			Out

			Values []string `group:"decoratedVals"`
		}

		type InvokeIn struct {
			In

			Values []string `group:"decoratedVals"`
		}

		parent := New()

		require.NoError(t, parent.Provide(func() string { return "dog" }, Group("values")))
		require.NoError(t, parent.Provide(func() string { return "cat" }, Group("values")))

		child := parent.Scope("child")

		parent.Decorate(func(i DecorateIn) DecorateOut {
			var result []string
			for _, val := range i.Values {
				result = append(result, "happy "+val)
			}
			return DecorateOut{
				Values: result,
			}
		})

		child.Decorate(func(i DecorateIn) DecorateOut {
			var result []string
			for _, val := range i.Values {
				result = append(result, "good "+val)
			}
			return DecorateOut{
				Values: result,
			}
		})

		child.Invoke(func(i InvokeIn) {
			assert.ElementsMatch(t, []string{"happy dog", "happy cat", "good dog", "good cat"}, i.Values)
		})
	})
}

func TestDecorateFailure(t *testing.T) {
	t.Run("decorate a type that wasn't provided", func(t *testing.T) {
		t.Parallel()

		c := New()
		type A struct {
			Name string
		}

		require.NoError(t, c.Decorate(func(a *A) *A { return &A{Name: a.Name + "'"} }))
		err := c.Invoke(func(a *A) string { return a.Name })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing type: *dig.A")
	})

	t.Run("decorate the same type twice", func(t *testing.T) {
		t.Parallel()

		c := New()
		type A struct {
			Name string
		}
		require.NoError(t, c.Provide(func() *A { return &A{Name: "A"} }))
		require.NoError(t, c.Decorate(func(a *A) *A { return &A{Name: a.Name + "'"} }), "first decorate should not fail.")

		err := c.Decorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		require.Error(t, err, "expected second call to decorate to fail.")
		assert.Contains(t, err.Error(), "decorating *dig.A multiple times")
	})

	t.Run("decorator returns an error", func(t *testing.T) {
		t.Parallel()

		c := New()

		type A struct {
			Name string
		}

		require.NoError(t, c.Provide(func() *A { return &A{Name: "A"} }))
		require.NoError(t, c.Decorate(func(a *A) (*A, error) { return a, errors.New("great sadness") }))

		err := c.Invoke(func(a *A) {})
		require.Error(t, err, "expected the decorator to error out")
		assert.Contains(t, err.Error(), "failed to build *dig.A: great sadness")
	})

	t.Run("one of the decorators dependencies returns an error", func(t *testing.T) {
		t.Parallel()
		type DecorateIn struct {
			In
			Values []string `group:"value"`
		}
		type DecorateOut struct {
			Out
			Values []string `group:"decoratedVal"`
		}
		type InvokeIn struct {
			In
			Values []string `group:"decoratedVal"`
		}

		c := New()
		require.NoError(t, c.Provide(func() (string, error) {
			return "value 1", nil
		}, Group("value")))
		require.NoError(t, c.Provide(func() (string, error) {
			return "value 2", nil
		}, Group("value")))
		require.NoError(t, c.Provide(func() (string, error) {
			return "value 3", errors.New("sadness")
		}, Group("value")))

		require.NoError(t, c.Decorate(func(i DecorateIn) DecorateOut {
			return DecorateOut{Values: i.Values}
		}))

		err := c.Invoke(func(c InvokeIn) {})
		require.Error(t, err, "expected one of the group providers for a decorator to fail")
		assert.Contains(t, err.Error(), `could not build value group string[group="decoratedVal"]`)
		assert.Contains(t, err.Error(), `received non-nil error from function "go.uber.org/dig".TestDecorateFailure.func4.3`)
	})

	t.Run("use dig.Out parameter for decorator", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			Out

			Value string `name:"val"`
		}

		c := New()
		require.NoError(t, c.Provide(func() string { return "hello" }, Name("val")))
		err := c.Decorate(func(p Param) string { return "fail" })
		require.Error(t, err, "expected dig.Out struct used as param to fail")
		assert.Contains(t, err.Error(), "cannot depend on result objects")
	})

	t.Run("use dig.In as out parameter for decorator", func(t *testing.T) {
		t.Parallel()

		type Result struct {
			In

			Value string `name:"val"`
		}

		c := New()
		err := c.Decorate(func() Result { return Result{Value: "hi"} })
		require.Error(t, err, "expected dig.In struct used as result to fail")
		assert.Contains(t, err.Error(), "cannot provide parameter object")
	})

	t.Run("missing dependency for a decorator", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			In

			Value string `name:"val"`
		}

		c := New()
		require.NoError(t, c.Decorate(func(p Param) string { return p.Value }))
		err := c.Invoke(func(s string) {})
		require.Error(t, err, "expected missing dep check to fail the decorator")
		assert.Contains(t, err.Error(), `missing dependencies for function "go.uber.org/dig".TestDecorateFailure.func7.2`)
	})

	t.Run("duplicate decoration through value groups", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			In

			Value string `name:"val"`
		}
		type A struct {
			Name string
		}
		type Result struct {
			Out

			Value *A
		}

		c := New()
		require.NoError(t, c.Provide(func() string { return "value" }, Name("val")))
		require.NoError(t, c.Decorate(func(p Param) *A {
			return &A{
				Name: p.Value,
			}
		}))
		err := c.Decorate(func(p Param) Result {
			return Result{
				Value: &A{
					Name: p.Value,
				},
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot decorate using function func(dig.Param) dig.Result")
		assert.Contains(t, err.Error(), "decorating *dig.A multiple times")
	})
}

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

package dig_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"go.uber.org/dig/internal/digtest"
)

func TestDecorateSuccess(t *testing.T) {
	t.Run("simple decorate without names or groups", func(t *testing.T) {
		t.Parallel()
		type A struct {
			Name string
		}

		c := digtest.New(t)
		c.RequireProvide(func() *A { return &A{Name: "A"} })

		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A", a.Name, "expected name to not be decorated yet.")
		})

		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })

		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.Name, "expected name to equal decorated name.")
		})
	})

	t.Run("simple decorate a provider from child scope", func(t *testing.T) {
		t.Parallel()
		type A struct {
			Name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		child.RequireProvide(func() *A { return &A{Name: "A"} }, dig.Export(true))

		child.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A", a.Name, "expected name to equal original name in parent scope")
		})

		child.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.Name, "expected name to equal decorated name in child scope")
		})
	})

	t.Run("check parent-provided decorator doesn't need parent to invoke", func(t *testing.T) {
		type A struct {
			Name string
		}

		type B struct {
			dig.In

			Values []string `group:"values"`
		}
		type C struct {
			dig.Out

			Values []string `group:"values"`
		}

		c := digtest.New(t)
		child := c.Scope("child")

		c.RequireProvide(func() *A { return &A{Name: "A"} }, dig.Export(true))
		c.RequireProvide(func() string { return "val1" }, dig.Export(true), dig.Group("values"))
		c.RequireProvide(func() string { return "val2" }, dig.Export(true), dig.Group("values"))
		c.RequireProvide(func() string { return "val3" }, dig.Export(true), dig.Group("values"))
		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		c.RequireDecorate(func(b B) C {
			var val []string
			for _, v := range b.Values {
				val = append(val, v+"'")
			}
			return C{
				Values: val,
			}
		})
		child.RequireInvoke(func(a *A, b B) {
			assert.Equal(t, "A'", a.Name, "expected name to equal decorated name in child scope")
			assert.ElementsMatch(t, []string{"val1'", "val2'", "val3'"}, b.Values)
		})
	})

	t.Run("simple decorate a provider to a scope and its descendants", func(t *testing.T) {
		t.Parallel()
		type A struct {
			Name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		c.RequireProvide(func() *A { return &A{Name: "A"} })

		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		assertDecoratedName := func(a *A) {
			assert.Equal(t, a.Name, "A'", "expected name to equal decorated name")
		}
		c.RequireInvoke(assertDecoratedName)
		child.RequireInvoke(assertDecoratedName)
	})

	t.Run("modifications compose with descendants", func(t *testing.T) {
		t.Parallel()
		type A struct {
			Name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		c.RequireProvide(func() *A { return &A{Name: "A"} })

		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		child.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })

		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.Name, "expected decorated name in parent")
		})

		child.RequireInvoke(func(a *A) {
			assert.Equal(t, "A''", a.Name, "expected double-decorated name in child")
		})

		sibling := c.Scope("sibling")
		grandchild := child.Scope("grandchild")
		require.NoError(t, sibling.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.Name, "expected single-decorated name in sibling")
		}))
		require.NoError(t, grandchild.Invoke(func(a *A) {
			assert.Equal(t, "A''", a.Name, "expected double-decorated name in grandchild")
		}))
	})

	t.Run("decorate with In struct", func(t *testing.T) {
		t.Parallel()

		type A struct {
			Name string
		}
		type B struct {
			dig.In

			A *A
			B string `name:"b"`
		}

		type C struct {
			dig.Out

			A *A
			B string `name:"b"`
		}

		c := digtest.New(t)
		c.RequireProvide(func() *A { return &A{Name: "A"} })
		c.RequireProvide(func() string { return "b" }, dig.Name("b"))

		c.RequireDecorate(func(b B) C {
			return C{
				A: &A{
					Name: b.A.Name + "'",
				},
				B: b.B + "'",
			}
		})

		c.RequireInvoke(func(b B) {
			assert.Equal(t, "A'", b.A.Name)
			assert.Equal(t, "b'", b.B)
		})
	})

	t.Run("decorate with value groups", func(t *testing.T) {
		type Params struct {
			dig.In

			Animals []string `group:"animals"`
		}

		type Result struct {
			dig.Out

			Animals []string `group:"animals"`
		}

		c := digtest.New(t)
		c.RequireProvide(func() string { return "dog" }, dig.Group("animals"))
		c.RequireProvide(func() string { return "cat" }, dig.Group("animals"))
		c.RequireProvide(func() string { return "gopher" }, dig.Group("animals"))

		c.RequireDecorate(func(p Params) Result {
			animals := p.Animals
			for i := 0; i < len(animals); i++ {
				animals[i] = "good " + animals[i]
			}
			return Result{
				Animals: animals,
			}
		})

		c.RequireInvoke(func(p Params) {
			assert.ElementsMatch(t, []string{"good dog", "good cat", "good gopher"}, p.Animals)
		})
	})

	t.Run("decorate with optional parameter", func(t *testing.T) {
		c := digtest.New(t)

		type A struct{}
		type Param struct {
			dig.In

			Values []string `group:"values"`
			A      *A       `optional:"true"`
		}

		type Result struct {
			dig.Out

			Values []string `group:"values"`
		}

		c.RequireProvide(func() string { return "a" }, dig.Group("values"))
		c.RequireProvide(func() string { return "b" }, dig.Group("values"))

		c.RequireDecorate(func(p Param) Result {
			return Result{
				Values: append(p.Values, "c"),
			}
		})

		c.RequireInvoke(func(p Param) {
			assert.Equal(t, 3, len(p.Values))
			assert.ElementsMatch(t, []string{"a", "b", "c"}, p.Values)
			assert.Nil(t, p.A)
		})
	})

	t.Run("replace a type completely", func(t *testing.T) {
		t.Parallel()

		c := digtest.New(t)
		type A struct {
			From string
		}

		c.RequireProvide(func() A {
			assert.Fail(t, "provider shouldn't be called")
			return A{From: "provider"}
		})

		c.RequireDecorate(func() A {
			return A{From: "decorator"}
		})

		c.RequireInvoke(func(a A) {
			assert.Equal(t, a.From, "decorator", "value should be from decorator")
		})
	})

	t.Run("group value decorator from parent and child", func(t *testing.T) {
		type DecorateIn struct {
			dig.In

			Values []string `group:"decoratedVals"`
		}

		type DecorateOut struct {
			dig.Out

			Values []string `group:"decoratedVals"`
		}

		type InvokeIn struct {
			dig.In

			Values []string `group:"decoratedVals"`
		}

		parent := digtest.New(t)

		parent.RequireProvide(func() string { return "dog" }, dig.Group("decoratedVals"))
		parent.RequireProvide(func() string { return "cat" }, dig.Group("decoratedVals"))

		child := parent.Scope("child")

		require.NoError(t, parent.Decorate(func(i DecorateIn) DecorateOut {
			var result []string
			for _, val := range i.Values {
				result = append(result, "happy "+val)
			}
			return DecorateOut{
				Values: result,
			}
		}))

		require.NoError(t, child.Decorate(func(i DecorateIn) DecorateOut {
			var result []string
			for _, val := range i.Values {
				result = append(result, "good "+val)
			}
			return DecorateOut{
				Values: result,
			}
		}))

		require.NoError(t, child.Invoke(func(i InvokeIn) {
			assert.ElementsMatch(t, []string{"good happy dog", "good happy cat"}, i.Values)
		}))
	})

	t.Run("decorate a value group with an empty slice", func(t *testing.T) {
		type A struct {
			dig.In

			Values []string `group:"decoratedVals"`
		}

		type B struct {
			dig.Out

			Values []string `group:"decoratedVals"`
		}

		c := digtest.New(t)

		c.RequireProvide(func() string { return "v1" }, dig.Group("decoratedVals"))
		c.RequireProvide(func() string { return "v2" }, dig.Group("decoratedVals"))

		c.RequireInvoke(func(a A) {
			assert.ElementsMatch(t, []string{"v1", "v2"}, a.Values)
		})

		c.RequireDecorate(func(a A) B {
			return B{
				Values: nil,
			}
		})

		c.RequireInvoke(func(a A) {
			assert.Nil(t, a.Values)
		})
	})
}

func TestDecorateFailure(t *testing.T) {
	t.Run("decorate a type that wasn't provided", func(t *testing.T) {
		t.Parallel()

		c := digtest.New(t)
		type A struct {
			Name string
		}

		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		err := c.Invoke(func(a *A) string { return a.Name })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing type: *dig_test.A")
	})

	t.Run("decorate the same type twice", func(t *testing.T) {
		t.Parallel()

		c := digtest.New(t)
		type A struct {
			Name string
		}
		c.RequireProvide(func() *A { return &A{Name: "A"} })
		c.RequireDecorate(func(a *A) *A { return &A{Name: a.Name + "'"} })

		err := c.Decorate(func(a *A) *A { return &A{Name: a.Name + "'"} })
		require.Error(t, err, "expected second call to decorate to fail.")
		assert.Contains(t, err.Error(), "*dig_test.A already decorated")
	})

	t.Run("decorator returns an error", func(t *testing.T) {
		t.Parallel()

		c := digtest.New(t)

		type A struct {
			Name string
		}

		c.RequireProvide(func() *A { return &A{Name: "A"} })
		c.RequireDecorate(func(a *A) (*A, error) { return a, errors.New("great sadness") })

		err := c.Invoke(func(a *A) {})
		require.Error(t, err, "expected the decorator to error out")
		assert.Contains(t, err.Error(), "failed to build *dig_test.A: great sadness")
	})

	t.Run("missing decorator dependency", func(t *testing.T) {
		t.Parallel()

		c := digtest.New(t)

		type A struct{}
		type B struct{}

		c.RequireProvide(func() A { return A{} })
		c.RequireDecorate(func(A, B) A {
			assert.Fail(t, "this function must never be called")
			return A{}
		})

		err := c.Invoke(func(A) {
			assert.Fail(t, "this function must never be called")
		})
		require.Error(t, err, "must not invoke if a dependency is missing")
		assert.Contains(t, err.Error(), "missing type: dig_test.B")
	})

	t.Run("one of the decorators dependencies returns an error", func(t *testing.T) {
		t.Parallel()
		type DecorateIn struct {
			dig.In
			Values []string `group:"value"`
		}
		type DecorateOut struct {
			dig.Out
			Values []string `group:"decoratedVal"`
		}
		type InvokeIn struct {
			dig.In
			Values []string `group:"decoratedVal"`
		}

		c := digtest.New(t)
		c.RequireProvide(func() (string, error) {
			return "value 1", nil
		}, dig.Group("value"))

		c.RequireProvide(func() (string, error) {
			return "value 2", nil
		}, dig.Group("value"))

		c.RequireProvide(func() (string, error) {
			return "value 3", errors.New("sadness")
		}, dig.Group("value"))

		c.RequireDecorate(func(i DecorateIn) DecorateOut {
			return DecorateOut{Values: i.Values}
		})

		err := c.Invoke(func(c InvokeIn) {})
		require.Error(t, err, "expected one of the group providers for a decorator to fail")
		assert.Contains(t, err.Error(), `could not build value group`)
		assert.Contains(t, err.Error(), `string[group="decoratedVal"]`)
	})

	t.Run("use dig.Out parameter for decorator", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			dig.Out

			Value string `name:"val"`
		}

		c := digtest.New(t)
		c.RequireProvide(func() string { return "hello" }, dig.Name("val"))
		err := c.Decorate(func(p Param) string { return "fail" })
		require.Error(t, err, "expected dig.Out struct used as param to fail")
		assert.Contains(t, err.Error(), "cannot depend on result objects")
	})

	t.Run("use dig.In as out parameter for decorator", func(t *testing.T) {
		t.Parallel()

		type Result struct {
			dig.In

			Value string `name:"val"`
		}

		c := digtest.New(t)
		err := c.Decorate(func() Result { return Result{Value: "hi"} })
		require.Error(t, err, "expected dig.In struct used as result to fail")
		assert.Contains(t, err.Error(), "cannot provide parameter object")
	})

	t.Run("missing dependency for a decorator", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			dig.In

			Value string `name:"val"`
		}

		c := digtest.New(t)
		c.RequireDecorate(func(p Param) string { return p.Value })
		err := c.Invoke(func(s string) {})
		require.Error(t, err, "expected missing dep check to fail the decorator")
		assert.Contains(t, err.Error(), `missing dependencies`)
	})

	t.Run("duplicate decoration through value groups", func(t *testing.T) {
		t.Parallel()

		type Param struct {
			dig.In

			Value string `name:"val"`
		}
		type A struct {
			Name string
		}
		type Result struct {
			dig.Out

			Value *A
		}

		c := digtest.New(t)
		c.RequireProvide(func() string { return "value" }, dig.Name("val"))
		c.RequireDecorate(func(p Param) *A {
			return &A{
				Name: p.Value,
			}
		})

		err := c.Decorate(func(p Param) Result {
			return Result{
				Value: &A{
					Name: p.Value,
				},
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot decorate")
		assert.Contains(t, err.Error(), "function func(dig_test.Param) dig_test.Result")
		assert.Contains(t, err.Error(), "*dig_test.A already decorated")
	})
}

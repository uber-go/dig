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
	"fmt"
	"strings"
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
			name string
		}

		c := digtest.New(t)
		c.RequireProvide(func() *A { return &A{name: "A"} })

		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A", a.name, "expected name to not be decorated yet.")
		})

		var info dig.DecorateInfo

		c.RequireDecorate(func(a *A) *A { return &A{name: a.name + "'"} }, dig.FillDecorateInfo(&info))
		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected name to equal decorated name.")
		})

		require.Equal(t, len(info.Inputs), 1)
		require.Equal(t, len(info.Outputs), 1)
		assert.Equal(t, "*dig_test.A", info.Inputs[0].String())
		assert.Equal(t, "*dig_test.A", info.Outputs[0].String())
	})

	t.Run("simple decorate a provider from child scope", func(t *testing.T) {
		t.Parallel()
		type A struct {
			name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		child.RequireProvide(func() *A { return &A{name: "A"} }, dig.Export(true))

		var info dig.DecorateInfo
		child.RequireDecorate(func(a *A) *A { return &A{name: a.name + "'"} }, dig.FillDecorateInfo(&info))
		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A", a.name, "expected name to equal original name in parent scope")
		})

		child.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected name to equal decorated name in child scope")
		})

		require.Equal(t, len(info.Inputs), 1)
		require.Equal(t, len(info.Outputs), 1)
		assert.Equal(t, "*dig_test.A", info.Inputs[0].String())
		assert.Equal(t, "*dig_test.A", info.Outputs[0].String())
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

		c.RequireProvide(func() *A { return &A{Name: "A"} })
		c.RequireProvide(func() string { return "val1" }, dig.Group("values"))
		c.RequireProvide(func() string { return "val2" }, dig.Group("values"))
		c.RequireProvide(func() string { return "val3" }, dig.Group("values"))
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

	t.Run("value groups with multiple decorations", func(t *testing.T) {
		type params struct {
			dig.In

			Strings []string `group:"strings"`
		}

		type childResult struct {
			dig.Out

			Strings []string `group:"strings"`
		}

		type A []string

		parent := digtest.New(t)
		parent.RequireProvide(func() string { return "a" }, dig.Group("strings"))
		parent.RequireProvide(func() string { return "b" }, dig.Group("strings"))
		parent.RequireProvide(func() string { return "c" }, dig.Group("strings"))

		parent.RequireProvide(func(p params) A { return A(p.Strings) })

		child := parent.Scope("child")

		child.RequireDecorate(func(p params) childResult {
			res := childResult{Strings: make([]string, len(p.Strings))}
			for i, s := range p.Strings {
				res.Strings[i] = strings.ToUpper(s)
			}
			return res
		})
		child.RequireDecorate(func(p params) A {
			return append(A(p.Strings), "D")
		})

		require.NoError(t, child.Invoke(func(a A) {
			assert.ElementsMatch(t, A{"A", "B", "C", "D"}, a)
		}))
	})

	t.Run("simple decorate a provider to a scope and its descendants", func(t *testing.T) {
		type A struct {
			name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		c.RequireProvide(func() *A { return &A{name: "A"} })

		c.RequireDecorate(func(a *A) *A { return &A{name: a.name + "'"} })
		assertDecoratedName := func(a *A) {
			assert.Equal(t, a.name, "A'", "expected name to equal decorated name")
		}
		c.RequireInvoke(assertDecoratedName)
		child.RequireInvoke(assertDecoratedName)
	})

	t.Run("modifications compose with descendants", func(t *testing.T) {
		t.Parallel()
		type A struct {
			name string
		}

		c := digtest.New(t)
		child := c.Scope("child")
		c.RequireProvide(func() *A { return &A{name: "A"} })

		c.RequireDecorate(func(a *A) *A { return &A{name: a.name + "'"} })
		child.RequireDecorate(func(a *A) *A { return &A{name: a.name + "'"} })
		c.RequireInvoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected decorated name in parent")
		})

		child.RequireInvoke(func(a *A) {
			assert.Equal(t, "A''", a.name, "expected double-decorated name in child")
		})

		sibling := c.Scope("sibling")
		grandchild := child.Scope("grandchild")
		require.NoError(t, sibling.Invoke(func(a *A) {
			assert.Equal(t, "A'", a.name, "expected single-decorated name in sibling")
		}))
		require.NoError(t, grandchild.Invoke(func(a *A) {
			assert.Equal(t, "A''", a.name, "expected double-decorated name in grandchild")
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

		var info dig.DecorateInfo
		c.RequireDecorate(func(b B) C {
			return C{
				A: &A{
					Name: b.A.Name + "'",
				},
				B: b.B + "'",
			}
		}, dig.FillDecorateInfo(&info))

		c.RequireInvoke(func(b B) {
			assert.Equal(t, "A'", b.A.Name)
			assert.Equal(t, "b'", b.B)
		})

		require.Equal(t, 2, len(info.Inputs))
		require.Equal(t, 2, len(info.Outputs))
		assert.Equal(t, "*dig_test.A", info.Inputs[0].String())
		assert.Equal(t, `string[name = "b"]`, info.Inputs[1].String())
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

		var info dig.DecorateInfo
		c.RequireDecorate(func(p Params) Result {
			animals := p.Animals
			for i := 0; i < len(animals); i++ {
				animals[i] = "good " + animals[i]
			}
			return Result{
				Animals: animals,
			}
		}, dig.FillDecorateInfo(&info))

		c.RequireInvoke(func(p Params) {
			assert.ElementsMatch(t, []string{"good dog", "good cat", "good gopher"}, p.Animals)
		})

		require.Equal(t, 1, len(info.Inputs))
		assert.Equal(t, `[]string[group = "animals"]`, info.Inputs[0].String())
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

	t.Run("decorate two times within same scope", func(t *testing.T) {
		type A struct {
			name string
		}
		parent := digtest.New(t)
		parent.RequireProvide(func() string { return "parent" })
		parent.RequireProvide(func(n string) A { return A{name: n} })

		child := parent.Scope("child")

		child.RequireDecorate(func() string { return "child" })
		child.RequireDecorate(func(n string) A { return A{name: n + " decorated"} })

		require.NoError(t, child.Invoke(func(a A) {
			assert.Equal(t, "child decorated", a.name)
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

	t.Run("invoke with a transitive dependency on child-decorated exported type", func(t *testing.T) {
		type Inner struct {
			Int int
		}

		type Next struct {
			MyInner *Inner
		}

		c := digtest.New(t)
		child := c.Scope("child")

		child.RequireProvide(func() *Inner {
			return &Inner{Int: 42}
		}, dig.Export(true))
		child.RequireProvide(func(i *Inner) *Next {
			return &Next{MyInner: i}
		}, dig.Export(true))
		child.RequireDecorate(func() *Inner {
			return &Inner{Int: 5678}
		})
		c.RequireInvoke(func(n *Next) {
			assert.Equal(t, 5678, n.MyInner.Int)
		})
		child.RequireInvoke(func(n *Next) {
			assert.Equal(t, 5678, n.MyInner.Int)
		})
		c.RequireInvoke(func(i *Inner) {
			assert.Equal(t, 42, i.Int)
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

	t.Run("decorate value group with a single value", func(t *testing.T) {
		type A struct {
			dig.Out

			Value int `group:"val"`
		}

		root := digtest.New(t)

		root.RequireProvide(func() A { return A{Value: 1} })
		err := root.Decorate(func() A { return A{Value: 11} })

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decorating a value group requires decorating the entire value group")
	})
}

func TestMultipleDecorates(t *testing.T) {
	t.Run("decorate same type from parent and child, invoke child first", func(t *testing.T) {
		t.Parallel()
		type A struct{ value int }
		root := digtest.New(t)
		child := root.Scope("child")

		decorator := func(a A) A {
			return A{value: a.value + 1}
		}
		root.RequireProvide(func() A { return A{value: 0} })
		root.RequireDecorate(decorator)
		child.RequireDecorate(decorator)

		child.RequireInvoke(func(a A) {
			assert.Equal(t, 2, a.value)
		})
		root.RequireInvoke(func(a A) {
			assert.Equal(t, 1, a.value)
		})
	})

	t.Run("decorate same type from parent and child, invoke parent first", func(t *testing.T) {
		t.Parallel()
		type A struct{ value int }
		root := digtest.New(t)
		child := root.Scope("child")
		decorator := func(a A) A {
			return A{value: a.value + 1}
		}
		root.RequireProvide(func() A { return A{value: 0} })
		root.RequireDecorate(decorator)
		child.RequireDecorate(decorator)

		root.RequireInvoke(func(a A) {
			assert.Equal(t, 1, a.value)
		})
		child.RequireInvoke(func(a A) {
			assert.Equal(t, 2, a.value)
		})
	})

	t.Run("decorate same value group from parent and child, invoke child", func(t *testing.T) {
		t.Parallel()
		type A struct {
			dig.In

			Values []int `group:"val"`
		}
		type B struct {
			dig.Out

			Values []int `group:"val"`
		}

		root := digtest.New(t)
		child := root.Scope("child")
		decorator := func(a A) B {
			var newV []int
			for _, v := range a.Values {
				newV = append(newV, v+1)
			}
			return B{Values: newV}
		}

		// provide {0, 1, 2}
		root.Provide(func() int { return 0 }, dig.Group("val"))
		root.Provide(func() int { return 1 }, dig.Group("val"))
		root.Provide(func() int { return 2 }, dig.Group("val"))

		// decorate +1 to each element in parent
		root.RequireDecorate(decorator)

		// decorate +1 to each element in child
		child.RequireDecorate(decorator)

		child.RequireInvoke(func(a A) {
			assert.ElementsMatch(t, []int{2, 3, 4}, a.Values)
		})

		root.RequireInvoke(func(a A) {
			assert.ElementsMatch(t, []int{1, 2, 3}, a.Values)
		})
	})

	t.Run("decorate same value group from parent and child, invoke parent", func(t *testing.T) {
		t.Parallel()
		type A struct {
			dig.In

			Values []int `group:"val"`
		}
		type B struct {
			dig.Out

			Values []int `group:"val"`
		}

		root := digtest.New(t)
		child := root.Scope("child")
		decorator := func(a A) B {
			var newV []int
			for _, v := range a.Values {
				newV = append(newV, v+1)
			}
			return B{Values: newV}
		}

		// provide {0, 1, 2}
		root.Provide(func() int { return 0 }, dig.Group("val"))
		root.Provide(func() int { return 1 }, dig.Group("val"))
		root.Provide(func() int { return 2 }, dig.Group("val"))

		// decorate +1 to each element in parent
		root.RequireDecorate(decorator)

		// decorate +1 to each element in child
		child.RequireDecorate(decorator)

		root.Invoke(func(a A) {
			assert.ElementsMatch(t, []int{1, 2, 3}, a.Values)
		})

		child.Invoke(func(a A) {
			assert.ElementsMatch(t, []int{2, 3, 4}, a.Values)
		})
	})
}

func TestFillDecorateInfoString(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "FillDecorateInfo(0x0)", fmt.Sprint(dig.FillDecorateInfo(nil)))
	})

	t.Run("not nil", func(t *testing.T) {
		t.Parallel()

		opt := dig.FillDecorateInfo(new(dig.DecorateInfo))
		assert.NotEqual(t, fmt.Sprint(opt), "FillDecorateInfo(0x0)")
		assert.Contains(t, fmt.Sprint(opt), "FillDecorateInfo(0x")
	})
}

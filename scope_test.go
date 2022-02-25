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

package dig_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/dig"
	"go.uber.org/dig/internal/digtest"
)

func TestScopedOperations(t *testing.T) {
	t.Parallel()

	t.Run("private provides", func(t *testing.T) {
		c := digtest.New(t)
		s := c.Scope("child")
		type A struct{}

		f := func(a *A) {
			assert.NotEqual(t, nil, a)
		}

		s.RequireProvide(func() *A { return &A{} })
		s.RequireInvoke(f)
		assert.Error(t, c.Invoke(f))
	})

	t.Run("private provides inherits", func(t *testing.T) {
		type A struct{}
		type B struct{}

		useA := func(a *A) {
			assert.NotEqual(t, nil, a)
		}
		useB := func(b *B) {
			assert.NotEqual(t, nil, b)
		}

		c := digtest.New(t)
		c.RequireProvide(func() *A { return &A{} })

		child := c.Scope("child")
		child.RequireProvide(func() *B { return &B{} })
		child.RequireInvoke(useA)
		child.RequireInvoke(useB)

		grandchild := child.Scope("grandchild")

		grandchild.RequireInvoke(useA)
		grandchild.RequireInvoke(useB)
		assert.Error(t, c.Invoke(useB))
	})

	t.Run("provides to top-level Container propogates to all scopes", func(t *testing.T) {
		type A struct{}

		// Scope tree:
		//     root  <-- Provide(func() *A)
		//    /    \
		//   c1	    c2
		//   |     /  \
		//   gc1  gc2  gc3
		var allScopes []*digtest.Scope
		root := digtest.New(t)

		allScopes = append(allScopes, root.Scope("child 1"), root.Scope("child 2"))
		allScopes = append(allScopes, allScopes[0].Scope("grandchild 1"), allScopes[1].Scope("grandchild 2"), allScopes[1].Scope("grandchild 3"))

		root.RequireProvide(func() *A {
			return &A{}
		})

		// top-level provide should be available in all the scopes.
		for _, scope := range allScopes {
			scope.RequireInvoke(func(a *A) {})
		}
	})

	t.Run("provide with Export", func(t *testing.T) {
		// Scope tree:
		//     root
		//    /    \
		//   c1	    c2
		//   |     /  \
		//   gc1  gc2  gc3 <-- Provide(func() *A)

		root := digtest.New(t)
		var allScopes []*digtest.Scope

		allScopes = append(allScopes, root.Scope("child 1"), root.Scope("child 2"))
		allScopes = append(allScopes, allScopes[0].Scope("grandchild 1"), allScopes[1].Scope("grandchild 2"), allScopes[1].Scope("grandchild 3"))

		type A struct{}
		// provide to the leaf Scope with Export option set.
		allScopes[len(allScopes)-1].RequireProvide(func() *A {
			return &A{}
		}, dig.Export(true))

		// since constructor was provided with Export option, this should let all the Scopes below should see it.
		for _, scope := range allScopes {
			scope.RequireInvoke(func(a *A) {})
		}
	})

	t.Run("parent shares values with children", func(t *testing.T) {
		type (
			T1 struct{ s string }
			T2 struct{}
		)

		parent := digtest.New(t)

		parent.RequireProvide(func() T1 {
			assert.Fail(t, "parent should not be called")
			return T1{"parent"}
		})

		child := parent.Scope("child")

		var childCalled bool
		defer func() {
			assert.True(t, childCalled, "child constructor must be called")
		}()
		child.RequireProvide(func() T1 {
			childCalled = true
			return T1{"child"}
		})

		child.RequireProvide(func(v T1) T2 {
			assert.Equal(t, "child", v.s,
				"value should be built by child")
			return T2{}
		})

		child.RequireInvoke(func(T2) {})
	})
}

func TestScopeFailures(t *testing.T) {
	t.Parallel()

	t.Run("introduce a cycle with child", func(t *testing.T) {
		// what root sees:
		// A <- B    C
		// |         ^
		// |_________|
		//
		// what child sees:
		// A <- B <- C
		// |         ^
		// |_________|
		type A struct{}
		type B struct{}
		type C struct{}
		newA := func(*C) *A { return &A{} }
		newB := func(*A) *B { return &B{} }
		newC := func(*B) *C { return &C{} }

		// Create a child Scope, and introduce a cycle
		// in the child only.
		check := func(c *digtest.Container, fails bool) {
			s := c.Scope("child")
			c.RequireProvide(newA)
			s.RequireProvide(newB)
			err := c.Provide(newC)

			if fails {
				assert.Error(t, err, "expected a cycle to be introduced in the child")
				assert.Contains(t, err.Error(), `[scope "child"]`)
			} else {
				assert.NoError(t, err)
			}
		}

		// Same as check, but this time child should inherit
		// parent-provided constructors upon construction.
		checkWithInheritance := func(c *digtest.Container, fails bool) {
			c.RequireProvide(newA)
			s := c.Scope("child")
			s.RequireProvide(newB)
			err := c.Provide(newC)
			if fails {
				assert.Error(t, err, "expected a cycle to be introduced in the child")
				assert.Contains(t, err.Error(), `[scope "child"]`)
			} else {
				assert.NoError(t, err)
			}
		}

		// Test using different permutations
		nodeferContainers := []func() *digtest.Container{
			func() *digtest.Container { return digtest.New(t) },
			func() *digtest.Container { return digtest.New(t, dig.DryRun(true)) },
			func() *digtest.Container { return digtest.New(t, dig.DryRun(false)) },
		}
		// Container permutations with DeferAcyclicVerification.
		deferredContainers := []func() *digtest.Container{
			func() *digtest.Container { return digtest.New(t, dig.DeferAcyclicVerification()) },
			func() *digtest.Container { return digtest.New(t, dig.DeferAcyclicVerification(), dig.DryRun(true)) },
			func() *digtest.Container { return digtest.New(t, dig.DeferAcyclicVerification(), dig.DryRun(false)) },
		}

		for _, c := range nodeferContainers {
			check(c(), true)
			checkWithInheritance(c(), true)
		}

		// with deferAcyclicVerification, these should not
		// error on Provides.
		for _, c := range deferredContainers {
			check(c(), false)
			checkWithInheritance(c(), false)
		}
	})

	t.Run("introduce a cycle with Export option", func(t *testing.T) {
		// what root and child1 sees:
		// A <- B    C
		// |         ^
		// |_________|
		//
		// what child2 sees:
		// A <- B <- C
		// |         ^
		// |_________|

		type A struct{}
		type B struct{}
		type C struct{}
		newA := func(*C) *A { return &A{} }
		newB := func(*A) *B { return &B{} }
		newC := func(*B) *C { return &C{} }

		root := digtest.New(t)
		child1 := root.Scope("child 1")
		child2 := root.Scope("child 2")

		// A <- B made available to all Scopes with root provision.
		root.RequireProvide(newA)

		// B <- C made available to only child 2 with private provide.
		child2.RequireProvide(newB)

		// C <- A made available to all Scopes with Export provide.
		err := child1.Provide(newC, dig.Export(true))
		assert.Error(t, err, "expected a cycle to be introduced in child 2")
		assert.Contains(t, err.Error(), `[scope "child 2"]`)
	})

	t.Run("private provides do not propagate upstream", func(t *testing.T) {
		type A struct{}

		root := digtest.New(t)
		c := root.Scope("child")
		gc := c.Scope("grandchild")
		gc.RequireProvide(func() *A { return &A{} })

		assert.Error(t, root.Invoke(func(a *A) {}), "invoking on grandchild's private-provided type should fail")
		assert.Error(t, c.Invoke(func(a *A) {}), "invoking on child's private-provided type should fail")
	})

	t.Run("private provides to child should be available to grandchildren, but not root", func(t *testing.T) {
		type A struct{}
		// Scope tree:
		//     root
		//      |
		//     child  <-- Provide(func() *A)
		//     /  \
		//   gc1   gc2
		root := digtest.New(t)
		c := root.Scope("child")
		gc := c.Scope("grandchild")

		c.RequireProvide(func() *A { return &A{} })

		err := root.Invoke(func(a *A) {})
		assert.Error(t, err, "expected Invoke in root container on child's private-provided type to fail")
		assert.Contains(t, err.Error(), "missing type: *dig_test.A")

		gc.RequireInvoke(func(a *A) {})
	})
}

func TestScopeValueGroups(t *testing.T) {
	t.Run("provide in parent and child", func(t *testing.T) {
		type result struct {
			dig.Out

			Value string `group:"foo"`
		}

		root := digtest.New(t)
		root.RequireProvide(func() result {
			return result{Value: "a"}
		})

		root.RequireProvide(func() result {
			return result{Value: "b"}
		})

		root.RequireProvide(func() result {
			return result{Value: "c"}
		})

		child := root.Scope("child")
		child.RequireProvide(func() result {
			return result{Value: "d"}
		})

		type param struct {
			dig.In

			Values []string `group:"foo"`
		}

		t.Run("invoke parent", func(t *testing.T) {
			root.RequireInvoke(func(i param) {
				assert.ElementsMatch(t, []string{"a", "b", "c"}, i.Values)
			})

		})

		t.Run("invoke child", func(t *testing.T) {
			child.RequireInvoke(func(i param) {
				assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, i.Values)
			})

		})
	})

	t.Run("value group as a parent dependency", func(t *testing.T) {
		// Tree:
		//
		//   root      defines a function that consumes the value group
		//    |
		//    |
		//   child     produces values to the value group

		type T1 struct{}
		type param struct {
			dig.In

			Values []string `group:"foo"`
		}

		root := digtest.New(t)

		root.RequireProvide(func(p param) T1 {
			assert.ElementsMatch(t, []string{"a", "b", "c"}, p.Values)
			return T1{}
		})

		child := root.Scope("child")
		child.RequireProvide(func() string { return "a" }, dig.Group("foo"), dig.Export(true))
		child.RequireProvide(func() string { return "b" }, dig.Group("foo"), dig.Export(true))
		child.RequireProvide(func() string { return "c" }, dig.Group("foo"), dig.Export(true))

		// Invocation in child should see values provided to the child,
		// even though the constructor we're invoking is provided in
		// the parent.
		child.RequireInvoke(func(T1) {})
	})
}

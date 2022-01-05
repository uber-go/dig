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

func TestScopedOperations(t *testing.T) {
	t.Parallel()

	t.Run("getStores/ScopesFromRoot returns scopes from root in order of distance from root", func(t *testing.T) {
		c := New()
		s1 := c.Scope("child1")
		s2 := s1.Scope("child2")
		s3 := s2.Scope("child2")

		assert.Equal(t, []containerStore{s3, s2, s1, c.scope}, s3.storesToRoot())
		assert.Equal(t, []*Scope{s3, s2, s1, c.scope}, s3.ancestors())
	})

	t.Run("private provides", func(t *testing.T) {
		c := New()
		s := c.Scope("child")
		type A struct{}

		f := func(a *A) {
			assert.NotEqual(t, nil, a)
		}

		require.NoError(t, s.Provide(func() *A { return &A{} }))
		assert.NoError(t, s.Invoke(f))
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

		c := New()
		require.NoError(t, c.Provide(func() *A { return &A{} }))

		child := c.Scope("child")
		require.NoError(t, child.Provide(func() *B { return &B{} }))
		assert.NoError(t, child.Invoke(useA))
		assert.NoError(t, child.Invoke(useB))

		grandchild := child.Scope("grandchild")

		assert.NoError(t, grandchild.Invoke(useA))
		assert.NoError(t, grandchild.Invoke(useB))
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
		var allScopes []*Scope
		root := New()

		allScopes = append(allScopes, root.Scope("child 1"), root.Scope("child 2"))
		allScopes = append(allScopes, allScopes[0].Scope("grandchild 1"), allScopes[1].Scope("grandchild 2"), allScopes[1].Scope("grandchild 3"))

		require.NoError(t, root.Provide(func() *A {
			return &A{}
		}))

		// top-level provide should be available in all the scopes.
		for _, scope := range allScopes {
			assert.NoError(t, scope.Invoke(func(a *A) {}))
		}
	})

	t.Run("provide with Export", func(t *testing.T) {
		// Scope tree:
		//     root
		//    /    \
		//   c1	    c2
		//   |     /  \
		//   gc1  gc2  gc3 <-- Provide(func() *A)

		root := New()
		var allScopes []*Scope

		allScopes = append(allScopes, root.Scope("child 1"), root.Scope("child 2"))
		allScopes = append(allScopes, allScopes[0].Scope("grandchild 1"), allScopes[1].Scope("grandchild 2"), allScopes[1].Scope("grandchild 3"))

		type A struct{}
		// provide to the leaf Scope with Export option set.
		require.NoError(t, allScopes[len(allScopes)-1].Provide(func() *A {
			return &A{}
		}, Export(true)))

		// since constructor was provided with Export option, this should let all the Scopes below should see it.
		for _, scope := range allScopes {
			assert.NoError(t, scope.Invoke(func(a *A) {}))
		}
	})

	t.Run("parent shares values with children", func(t *testing.T) {
		type (
			T1 struct{ s string }
			T2 struct{}
		)

		parent := New()

		require.NoError(t, parent.Provide(func() T1 {
			assert.Fail(t, "parent should not be called")
			return T1{"parent"}
		}))

		child := parent.Scope("child")

		var childCalled bool
		defer func() {
			assert.True(t, childCalled, "child constructor must be called")
		}()
		require.NoError(t, child.Provide(func() T1 {
			childCalled = true
			return T1{"child"}
		}))

		require.NoError(t, child.Provide(func(v T1) T2 {
			assert.Equal(t, "child", v.s,
				"value should be built by child")
			return T2{}
		}))

		require.NoError(t, child.Invoke(func(T2) {}))
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
		check := func(c *Container, fails bool) {
			s := c.Scope("child")
			assert.NoError(t, c.Provide(newA))
			assert.NoError(t, s.Provide(newB))
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
		checkWithInheritance := func(c *Container, fails bool) {
			assert.NoError(t, c.Provide(newA))
			s := c.Scope("child")
			assert.NoError(t, s.Provide(newB))
			err := c.Provide(newC)
			if fails {
				assert.Error(t, err, "expected a cycle to be introduced in the child")
				assert.Contains(t, err.Error(), `[scope "child"]`)
			} else {
				assert.NoError(t, err)
			}
		}

		// Test using different permutations
		nodeferContainers := []func() *Container{
			func() *Container { return New() },
			func() *Container { return New(DryRun(true)) },
			func() *Container { return New(DryRun(false)) },
		}
		// Container permutations with DeferAcyclicVerification.
		deferredContainers := []func() *Container{
			func() *Container { return New(DeferAcyclicVerification()) },
			func() *Container { return New(DeferAcyclicVerification(), DryRun(true)) },
			func() *Container { return New(DeferAcyclicVerification(), DryRun(false)) },
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

		root := New()
		child1 := root.Scope("child 1")
		child2 := root.Scope("child 2")

		// A <- B made available to all Scopes with root provision.
		require.NoError(t, root.Provide(newA))

		// B <- C made available to only child 2 with private provide.
		require.NoError(t, child2.Provide(newB))

		// C <- A made available to all Scopes with Export provide.
		err := child1.Provide(newC, Export(true))
		assert.Error(t, err, "expected a cycle to be introduced in child 2")
		assert.Contains(t, err.Error(), `[scope "child 2"]`)
	})

	t.Run("private provides do not propagate upstream", func(t *testing.T) {
		type A struct{}

		root := New()
		c := root.Scope("child")
		gc := c.Scope("grandchild")
		require.NoError(t, gc.Provide(func() *A { return &A{} }))

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
		root := New()
		c := root.Scope("child")
		gc := c.Scope("grandchild")

		require.NoError(t, c.Provide(func() *A { return &A{} }))

		err := root.Invoke(func(a *A) {})
		assert.Error(t, err, "expected Invoke in root container on child's private-provided type to fail")
		assert.Contains(t, err.Error(), "missing type: *dig.A")

		assert.NoError(t, gc.Invoke(func(a *A) {}), "expected Invoke in grandchild container on child's private-provided type to fail")
	})
}

func TestScopeValueGroups(t *testing.T) {
	t.Run("provide in parent and child", func(t *testing.T) {
		type result struct {
			Out

			Value string `group:"foo"`
		}

		root := New()
		require.NoError(t, root.Provide(func() result {
			return result{Value: "a"}
		}))
		require.NoError(t, root.Provide(func() result {
			return result{Value: "b"}
		}))
		require.NoError(t, root.Provide(func() result {
			return result{Value: "c"}
		}))

		child := root.Scope("child")
		require.NoError(t,
			child.Provide(func() result {
				return result{Value: "d"}
			}))

		type param struct {
			In

			Values []string `group:"foo"`
		}

		t.Run("invoke parent", func(t *testing.T) {
			require.NoError(t, root.Invoke(func(i param) {
				assert.ElementsMatch(t, []string{"a", "b", "c"}, i.Values)
			}), "only values added to parent should be visible")
		})

		t.Run("invoke child", func(t *testing.T) {
			require.NoError(t, child.Invoke(func(i param) {
				assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, i.Values)
			}), "values added to both, parent and child should be visible")
		})
	})
}

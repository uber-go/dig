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

		assert.Equal(t, []containerStore{c.scope, s1, s2, s3}, s3.getStoresFromRoot())
		assert.Equal(t, []*Scope{c.scope, s1, s2, s3}, s3.getScopesFromRoot())
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
		child.Provide(func() *B { return &B{} })
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

		root.Provide(func() *A {
			return &A{}
		})

		// top-level provide should be available in all the scopes.
		for _, scope := range allScopes {
			assert.NoError(t, scope.Invoke(func(a *A) {}))
		}
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

		c.Provide(func() *A { return &A{} })

		err := root.Invoke(func(a *A) {})
		assert.Error(t, err, "expected Invoke in root container on child's private-provided type to fail")
		assert.Contains(t, err.Error(), "missing type: *dig.A")

		assert.NoError(t, gc.Invoke(func(a *A) {}), "expected Invoke in grandchild container on child's private-provided type to fail")
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

		c := New()
		s := c.Scope("child")
		assert.NoError(t, c.Provide(newA))
		assert.NoError(t, s.Provide(newB))
		err := c.Provide(newC)
		assert.Error(t, err, "expected a cycle to be introduced in the child")
		assert.Contains(t, err.Error(), "In Scope child")

		// Try again, this time with child inheriting parent-provided constructors.
		c = New()
		assert.NoError(t, c.Provide(newA))
		s = c.Scope("child")
		assert.NoError(t, s.Provide(newB))
		err = c.Provide(newC)
		assert.Error(t, err, "expected a cycle to be introduced in the child")
		assert.Contains(t, err.Error(), "In Scope child")
	})

	t.Run("private provides do not propagate upstream", func(t *testing.T) {
		type A struct{}

		root := New()
		c := root.Scope("child")
		gc := c.Scope("grandchild")
		gc.Provide(func() *A { return &A{} })

		assert.Error(t, root.Invoke(func(a *A) {}), "invoking on grandchild's private-provided type should fail")
		assert.Error(t, c.Invoke(func(a *A) {}), "invoking on child's private-provided type should fail")
	})

}

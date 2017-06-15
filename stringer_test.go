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
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	t.Parallel()

	c := New()
	type A struct{}
	type B struct{}
	type C struct{}
	type D struct{}
	type param struct {
		In
		*D `named:"foo" optional:"true"`
	}
	require.NoError(t, c.Provide(func(p param) (*A, *B) { return &A{}, &B{} }))
	require.NoError(t, c.Provide(func(*B) *C { return &C{} }))
	require.NoError(t, c.Invoke(func(a *A) {}))

	b := &bytes.Buffer{}
	fmt.Fprintln(b, c)
	s := b.String()

	// all nodes are in the graph
	assert.Contains(t, s, "*dig.A -> deps: [~*dig.D]")
	assert.Contains(t, s, "*dig.B -> deps: [~*dig.D]")
	assert.Contains(t, s, "*dig.C -> deps: [*dig.B]")

	// constructors
	assert.Contains(t, s, "func(dig.param) (*dig.A, *dig.B)")
	assert.Contains(t, s, "func(*dig.B) *dig.C")

	// cache
	assert.Contains(t, s, "*dig.A =>")
	assert.Contains(t, s, "*dig.B =>")
}

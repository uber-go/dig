// Copyright (c) 2019 Uber Technologies, Inc.
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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	type A struct{}
	type B struct{}
	type C struct{}
	type D struct{}

	type in struct {
		In

		A A `name:"foo"`
		B B `optional:"true"`
		C C `name:"bar" optional:"true"`

		Strings []string `group:"baz"`
	}

	type out struct {
		Out

		A A `name:"foo"`
		C C `name:"bar"`
	}

	type stringOut struct {
		Out

		S string `group:"baz"`
	}

	c := New(setRand(rand.New(rand.NewSource(0))))

	require.NoError(t, c.Provide(func(i in) D {
		assert.Equal(t, []string{"bar", "baz", "foo"}, i.Strings)
		return D{}
	}))

	require.NoError(t, c.Provide(func() out {
		return out{
			A: A{},
			C: C{},
		}
	}))

	require.NoError(t, c.Provide(func() A { return A{} }))
	require.NoError(t, c.Provide(func() B { return B{} }))
	require.NoError(t, c.Provide(func() C { return C{} }))

	require.NoError(t, c.Provide(func(A) stringOut { return stringOut{S: "foo"} }))
	require.NoError(t, c.Provide(func(B) stringOut { return stringOut{S: "bar"} }))
	require.NoError(t, c.Provide(func(C) stringOut { return stringOut{S: "baz"} }))

	require.NoError(t, c.Invoke(func(D) {
	}))

	s := c.String()

	// All nodes
	assert.Contains(t, s, `dig.A[name="foo"] -> deps: []`)
	assert.Contains(t, s, "dig.A -> deps: []")
	assert.Contains(t, s, "dig.B -> deps: []")
	assert.Contains(t, s, "dig.C -> deps: []")
	assert.Contains(t, s, `dig.C[name="bar"] -> deps: []`)
	assert.Contains(t, s, `dig.D -> deps: [dig.A[name="foo"] dig.B[optional] dig.C[optional, name="bar"] string[group="baz"]]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig.A]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig.B]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig.C]`)

	// Values
	assert.Contains(t, s, "dig.A => {}")
	assert.Contains(t, s, "dig.B => {}")
	assert.Contains(t, s, "dig.C => {}")
	assert.Contains(t, s, "dig.D => {}")
	assert.Contains(t, s, `dig.A[name="foo"] => {}`)
	assert.Contains(t, s, `dig.C[name="bar"] => {}`)
	assert.Contains(t, s, `string[group="baz"] => foo`)
	assert.Contains(t, s, `string[group="baz"] => bar`)
	assert.Contains(t, s, `string[group="baz"] => baz`)
}

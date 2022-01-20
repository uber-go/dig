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

package dig_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/dig"
	"go.uber.org/dig/internal/digtest"
)

func TestStringer(t *testing.T) {
	type A struct{}
	type B struct{}
	type C struct{}
	type D struct{}

	type in struct {
		dig.In

		A A `name:"foo"`
		B B `optional:"true"`
		C C `name:"bar" optional:"true"`

		Strings []string `group:"baz"`
	}

	type out struct {
		dig.Out

		A A `name:"foo"`
		C C `name:"bar"`
	}

	type stringOut struct {
		dig.Out

		S string `group:"baz"`
	}

	c := digtest.New(t, dig.SetRand(rand.New(rand.NewSource(0))))

	c.RequireProvide(func(i in) D {
		assert.Equal(t, []string{"bar", "baz", "foo"}, i.Strings)
		return D{}
	})

	c.RequireProvide(func() out {
		return out{
			A: A{},
			C: C{},
		}
	})

	c.RequireProvide(func() A { return A{} })
	c.RequireProvide(func() B { return B{} })
	c.RequireProvide(func() C { return C{} })

	c.RequireProvide(func(A) stringOut { return stringOut{S: "foo"} })
	c.RequireProvide(func(B) stringOut { return stringOut{S: "bar"} })
	c.RequireProvide(func(C) stringOut { return stringOut{S: "baz"} })

	c.RequireInvoke(func(D) {
	})

	s := c.String()

	// All nodes
	assert.Contains(t, s, `dig_test.A[name="foo"] -> deps: []`)
	assert.Contains(t, s, "dig_test.A -> deps: []")
	assert.Contains(t, s, "dig_test.B -> deps: []")
	assert.Contains(t, s, "dig_test.C -> deps: []")
	assert.Contains(t, s, `dig_test.C[name="bar"] -> deps: []`)
	assert.Contains(t, s, `dig_test.D -> deps: [dig_test.A[name="foo"] dig_test.B[optional] dig_test.C[optional, name="bar"] string[group="baz"]]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig_test.A]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig_test.B]`)
	assert.Contains(t, s, `string[group="baz"] -> deps: [dig_test.C]`)

	// Values
	assert.Contains(t, s, "dig_test.A => {}")
	assert.Contains(t, s, "dig_test.B => {}")
	assert.Contains(t, s, "dig_test.C => {}")
	assert.Contains(t, s, "dig_test.D => {}")
	assert.Contains(t, s, `dig_test.A[name="foo"] => {}`)
	assert.Contains(t, s, `dig_test.C[name="bar"] => {}`)
	assert.Contains(t, s, `string[group="baz"] => foo`)
	assert.Contains(t, s, `string[group="baz"] => bar`)
	assert.Contains(t, s, `string[group="baz"] => baz`)
}

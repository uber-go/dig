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

// +build go1.9

package dig

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndSuccessWithAliases(t *testing.T) {
	t.Run("pointer constructor", func(t *testing.T) {
		type Buffer = *bytes.Buffer

		c := New()

		var b Buffer
		require.NoError(t, c.Provide(func() *bytes.Buffer {
			b = &bytes.Buffer{}
			return b
		}), "provide failed")

		require.NoError(t, c.Invoke(func(got Buffer) {
			require.NotNil(t, got, "invoke got nil buffer")
			require.True(t, got == b, "invoke got wrong buffer")
		}), "invoke failed")
	})

	t.Run("duplicate provide", func(t *testing.T) {
		type A struct{}
		type B = A

		c := New()
		require.NoError(t, c.Provide(func() A {
			return A{}
		}), "A should not fail to provide")

		err := c.Provide(func() B { return B{} })
		require.Error(t, err, "B should fail to provide")
		assert.Contains(t, err.Error(), `can't provide func() dig.A`)
		assert.Contains(t, err.Error(), `already in the container`)
	})

	t.Run("named instances", func(t *testing.T) {
		c := New()
		type A1 struct{ s string }
		type A2 = A1
		type A3 = A2

		type ret struct {
			Out

			A A1 `name:"a"`
			B A2 `name:"b"`
			C A3 `name:"c"`
		}

		type param struct {
			In

			A1 A1 `name:"a"`
			B1 A2 `name:"b"`
			C1 A3 `name:"c"`

			A2 A3 `name:"a"`
			B2 A1 `name:"b"`
			C2 A2 `name:"c"`

			A3 A2 `name:"a"`
			B3 A3 `name:"b"`
			C3 A1 `name:"c"`
		}
		require.NoError(t, c.Provide(func() ret {
			return ret{A: A2{"a"}, B: A3{"b"}, C: A1{"c"}}
		}), "provide for three named instances should succeed")

		require.NoError(t, c.Invoke(func(p param) {
			assert.Equal(t, "a", p.A1.s, "A1 should match")
			assert.Equal(t, "b", p.B1.s, "B1 should match")
			assert.Equal(t, "c", p.C1.s, "C1 should match")

			assert.Equal(t, "a", p.A2.s, "A2 should match")
			assert.Equal(t, "b", p.B2.s, "B2 should match")
			assert.Equal(t, "c", p.C2.s, "C2 should match")

			assert.Equal(t, "a", p.A3.s, "A3 should match")
			assert.Equal(t, "b", p.B3.s, "B3 should match")
			assert.Equal(t, "c", p.C3.s, "C3 should match")

		}), "invoke should succeed, pulling out two named instances")
	})

}

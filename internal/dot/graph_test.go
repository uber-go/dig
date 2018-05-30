// Copyright (c) 2018 Uber Technologies, Inc.
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

package dot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDotGraphAdd(t *testing.T) {
	n1 := &Node{Type: "t1", Name: "Type 1"}
	n2 := &Node{Type: "t2", Optional: true}
	n3 := &Node{Type: "t3", Name: "Type 3", Optional: true}
	n4 := &Node{Type: "t4", Group: "Group 4"}

	t.Run("Add empty param and result list", func(t *testing.T) {
		dg := new(Graph)
		dg.Add(make([]*Node, 0), make([]*Node, 0))

		assert.Equal(t, new(Graph), dg)
	})

	t.Run("Add param and result list", func(t *testing.T) {
		expected := new(Graph)
		expected.Edges = []*Edge{
			{Param: n1, Result: n3},
			{Param: n1, Result: n4},
			{Param: n2, Result: n3},
			{Param: n2, Result: n4},
		}

		dg := new(Graph)
		dg.Add([]*Node{n1, n2}, []*Node{n3, n4})

		assert.Equal(t, expected, dg)
	})
}

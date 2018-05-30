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
	n1 := &Node{Type: "t1", Name: "type1"}
	n2 := &Node{Type: "t2", Optional: true}
	n3 := &Node{Type: "t3", Name: "type3", Optional: true}
	n4 := &Node{Type: "t4", Group: "group4"}

	t.Run("Add param and result list", func(t *testing.T) {
		expected := new(Graph)
		expected.Ctors = []*Ctor{
			{Params: []*Node{n1, n2, n2, n2}, Results: []*Node{n3, n4}},
		}
		expected.Nodes = map[string]*Node{
			"t1[name=\"type1\"]":   n1,
			"t2":                   n2,
			"t3[name=\"type3\"]":   n3,
			"t4[group=\"group4\"]": n4,
		}

		dg := &Graph{Nodes: make(map[string]*Node)}
		dg.Add(&Ctor{}, []*Node{n1, n2, n2, n2}, []*Node{n3, n4})

		assert.Equal(t, expected, dg)
	})
}

func TestNodeStr(t *testing.T) {
	n1 := &Node{Type: "t1"}
	n2 := &Node{Type: "t2", Name: "bar"}
	n3 := &Node{Type: "t3", Group: "foo"}

	assert.Equal(t, "t1", n1.str())
	assert.Equal(t, "t2[name=\"bar\"]", n2.str())
	assert.Equal(t, "t3[group=\"foo\"]", n3.str())
}

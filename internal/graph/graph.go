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

package graph

// Iterator TODO
type Iterator interface {
	Order() int

	Visit(u int, do func(v int) bool)
}

// IsAcyclic TODO
func IsAcyclic(g Iterator) bool {
	// use topological sort to check if DAG is acyclic.
	degrees := make([]int, g.Order())
	var q []int

	for u := range degrees {
		g.Visit(u, func(v int) bool {
			degrees[v]++
			return false
		})
	}

	// find roots (nodes w/o any other nodes depending on it)
	// to determine where to start traversing the graph from.
	for u, deg := range degrees {
		if deg == 0 {
			q = append(q, u)
		}
	}

	vertexCount := 0
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		vertexCount++
		g.Visit(u, func(v int) bool {
			degrees[v]--
			if degrees[v] == 0 {
				q = append(q, v)
			}
			return false
		})
	}
	return vertexCount == g.Order()
}

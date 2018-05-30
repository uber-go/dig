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

// Node is a single node in a graph.
type Node struct {
	Type     string
	Name     string
	Optional bool
	Group    string
}

// Edge connects a node parameter to a node result.
type Edge struct {
	Param  *Node
	Result *Node
}

// Graph is the DOT-format graph in a Container.
type Graph struct {
	Edges []*Edge
}

// Add adds the edges in node n into dg.
func (dg *Graph) Add(params []*Node, results []*Node) {
	edges := make([]*Edge, 0, len(params)*len(results))

	for _, param := range params {
		for _, result := range results {
			edges = append(edges, &Edge{Param: param, Result: result})
		}
	}

	dg.Edges = append(dg.Edges, edges...)
}

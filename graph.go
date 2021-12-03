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

// graphNode represents a single node in the dependency graph's graph representation.
type graphNode struct {
	// The index of this node in the graphHolder's allNodes.
	Order   int
	Wrapped interface{}
}

// graphHolder represents the dependency graph for a Container. Specifically,
// it saves constructorNodes and paramGroupedSlices (value groups) as graphNodes
// and implements the Graph interface defined in internal/graph to run graph
// algorithms on it. It has a 1-to-1 correspondence with a Container whose graph
// it represents.
type graphHolder struct {
	// all the nodes defined in the graph.
	allNodes []*graphNode

	// Maps each graphNode to its index in allNodes slice.
	orders map[key]int

	// Container whose graph this holder contains.
	c *Container
}

func newGraphHolder(c *Container) *graphHolder {
	return &graphHolder{
		orders: make(map[key]int),
		c:      c,
		allNodes: []*graphNode{
			{
				Order:   0,
				Wrapped: nil,
			},
		},
	}

}

func (gh *graphHolder) Order() int {
	return len(gh.allNodes)
}

func (gh *graphHolder) EdgesFrom(u int) []int {
	n := gh.allNodes[u]

	var orders []int

	switch w := n.Wrapped.(type) {
	case *constructorNode:
		for _, param := range w.paramList.Params {
			orders = append(orders, getParamOrder(gh, param)...)
		}
	case *paramGroupedSlice:
		providers := gh.c.getGroupProviders(w.Group, w.Type.Elem())
		for _, provider := range providers {
			orders = append(orders, gh.orders[key{t: provider.CType()}])
		}
	}
	return orders
}

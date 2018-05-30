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
	"fmt"
)

// Ctor encodes the edges in a graph. It includes information like the name
// of the constructor, the package, file, and line where the constructor is provided,
// and the params and results of the constructor.
type Ctor struct {
	Name    string
	Package string
	File    string
	Line    int
	Params  []*Node
	Results []*Node
}

// Graph is the DOT-format graph in a Container represented by a list of Ctor-s.
type Graph struct {
	Ctors []*Ctor
	Nodes map[string]*Node
}

// Node is a single node in a graph.
type Node struct {
	Type     string
	Name     string
	Optional bool
	Group    string
}

// Add updates the param and result nodes in the constructor and adds the constructor to the graph
func (dg *Graph) Add(c *Ctor, params []*Node, results []*Node) {
	c.Params = getNodes(dg, params)
	c.Results = getNodes(dg, results)
	dg.Ctors = append(dg.Ctors, c)
}

func (n *Node) str() string {
	if n.Name != "" {
		return fmt.Sprintf("%v[name=%q]", n.Type, n.Name)
	} else if n.Group != "" {
		return fmt.Sprintf("%v[group=%q]", n.Type, n.Group)
	}

	return n.Type
}

func getNodes(dg *Graph, nodes []*Node) []*Node {
	ptrs := make([]*Node, len(nodes))
	for i, node := range nodes {
		k := node.str()

		if n := dg.Nodes[k]; n != nil {
			ptrs[i] = n
		} else {
			dg.Nodes[k] = node
			ptrs[i] = node
		}
	}
	return ptrs
}

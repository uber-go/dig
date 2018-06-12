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
	"reflect"
)

// Ctor encodes a constructor provided to the container for the DOT graph.
type Ctor struct {
	Name    string
	Package string
	File    string
	Line    int
	Params  []*Node
	Results []*Node
}

// Graph is the DOT-format graph in a Container.
type Graph struct {
	Ctors       []*Ctor
	ctorMap     map[key][]*Ctor
	nodes       map[key][]*Node
	subscribers map[key][]*Ctor
}

// Node is a single node in a graph.
type Node struct {
	Type       reflect.Type
	Name       string
	Optional   bool
	Group      string
	GroupIndex int
}

type key struct {
	t     reflect.Type
	name  string
	group string
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		ctorMap:     make(map[key][]*Ctor),
		nodes:       make(map[key][]*Node),
		subscribers: make(map[key][]*Ctor),
	}
}

// AddDotCtor adds a constructor to the graph, changing the params and results
// of the constructor to point at the nodes they refer to, and subscribing it
// to any grouped params.
func (dg *Graph) AddDotCtor(c *Ctor) {
	c.findParams(dg, c.Params)
	dg.addResults(c)
}

// String returns the string representation of a node so the different
// constructors can refer to the same node. We omit information on the optional
// field since the same type can be optional to one constructor and required
// for another.
func (n *Node) String() string {
	if n.Name != "" {
		return fmt.Sprintf("%v[name=%v]", n.Type.String(), n.Name)
	} else if n.Group != "" {
		return fmt.Sprintf("%v[group=%v]%v", n.Type.String(), n.Group, n.GroupIndex)
	}

	return n.Type.String()
}

// Attributes composes and returns a string to style the sublabels when
// visualizing graph.
func (n *Node) Attributes() string {
	switch {
	case n.Name != "":
		return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Name: %v</FONT>`, n.Name)
	case n.Group != "":
		return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Group: %v</FONT>`, n.Group)
	default:
		return ""
	}
}

func (dg *Graph) subscribe(k key, c *Ctor) {
	if dg.subscribers[k] == nil {
		dg.subscribers[k] = []*Ctor{c}
	} else {
		dg.subscribers[k] = append(dg.subscribers[k], c)
	}
}

func (dg *Graph) addResults(c *Ctor) {
	for i, node := range c.Results {
		k := key{t: node.Type, name: node.Name, group: node.Group}

		switch {
		case node.Group != "":
			if dg.nodes[k] == nil {
				dg.nodes[k] = []*Node{}
			}

			node.GroupIndex = len(dg.nodes[k])
			dg.nodes[k] = append(dg.nodes[k], node)

			if dg.subscribers[k] != nil {
				for _, ctor := range dg.subscribers[k] {
					ctor.Params = append(ctor.Params, node)
				}
			}
		case dg.nodes[k] == nil:
			dg.nodes[k] = []*Node{node}
		default:
			c.Results[i] = dg.nodes[k][0]
		}

		if dg.ctorMap[k] == nil {
			dg.ctorMap[k] = []*Ctor{c}
		} else {
			dg.ctorMap[k] = append(dg.ctorMap[k], c)
		}
	}
	dg.Ctors = append(dg.Ctors, c)
}

func (c *Ctor) findParams(dg *Graph, nodes []*Node) {
	ptrs := []*Node{}
	for _, node := range c.Params {
		var k key
		k = key{t: node.Type, name: node.Name, group: node.Group}

		if node.Group != "" {
			dg.subscribe(k, c)
		} else {
			if dg.nodes[k] == nil {
				dg.nodes[k] = []*Node{node}
			}
		}

		ptrs = append(ptrs, dg.nodes[k]...)
	}
	c.Params = ptrs
}

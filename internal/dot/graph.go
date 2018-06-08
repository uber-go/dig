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

import "fmt"

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
	Ctors []*Ctor
}

// Node is a single node in a graph.
type Node struct {
	Type     string
	Name     string
	Optional bool
	Group    string
}

// String returns the string representation of a node so the different
// constructors can refer to the same node. We omit information on the optional
// field since the same type can be optional to one constructor and required
// for another.
func (n *Node) String() string {
	if n.Name != "" {
		return fmt.Sprintf("%v[name=%v]", n.Type, n.Name)
	} else if n.Group != "" {
		return fmt.Sprintf("%v[group=%v]", n.Type, n.Group)
	}

	return n.Type
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

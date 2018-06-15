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
	Name        string
	Package     string
	File        string
	Line        int
	Params      []*Param
	GroupParams []*Group
	Results     []*Result
}

// Param is a parameter node in the graph.
type Param struct {
	*Node
	Optional bool
}

// Result is a result node in the graph.
type Result struct {
	*Node

	// GroupIndex is added to differenciate grouped values from one another.
	// Since grouped values have the same type and group, their Node / string
	// representations are the same so we need indices to uniquely identify
	// the values.
	GroupIndex int
}

// Group is a group node in the graph.
type Group struct {
	// Type is the type of values in the group.
	Type    reflect.Type
	Group   string
	Results []*Result
}

// Graph is the DOT-format graph in a Container.
type Graph struct {
	Ctors  []*Ctor
	Groups map[groupKey]*Group
}

// Node is a single node in a graph and is embedded into Params and Results.
type Node struct {
	Type  reflect.Type
	Name  string
	Group string
}

type groupKey struct {
	t     reflect.Type
	group string
}

// NewGraph creates an empty graph.
func NewGraph() *Graph {
	return &Graph{
		Groups: make(map[groupKey]*Group),
	}
}

// NewGroup creates a new group with information in the groupKey.
func NewGroup(k groupKey) *Group {
	return &Group{
		Type:  k.t,
		Group: k.group,
	}
}

// AddCtor adds the constructor with paramList and resultList into the graph.
func (dg *Graph) AddCtor(c *Ctor, paramList []*Param, resultList []*Result) {
	var (
		params      []*Param
		groupParams []*Group
	)

	// Loop through the paramList to separate them into regular params and
	// grouped params. For grouped params, we use getGroup to find the actual
	// group.
	for _, param := range paramList {
		if param.Group == "" {
			// Not a value group.
			params = append(params, param)
			continue
		}

		k := groupKey{t: param.Type.Elem(), group: param.Group}
		group := dg.getGroup(k)
		groupParams = append(groupParams, group)
	}

	for _, result := range resultList {
		// If the result is a grouped value, we want to update its GroupIndex
		// and add it to the Group.
		if result.Group != "" {
			dg.addToGroup(result)
		}
	}

	c.Params = params
	c.GroupParams = groupParams
	c.Results = resultList

	dg.Ctors = append(dg.Ctors, c)
}

// getGroup finds the group by groupKey from the graph. If it is not available,
// a new group is created and returned.
func (dg *Graph) getGroup(k groupKey) *Group {
	g, ok := dg.Groups[k]
	if !ok {
		g = NewGroup(k)
		dg.Groups[k] = g
	}
	return g
}

// addToGroup adds a newly provided grouped result to the appropriate group.
func (dg *Graph) addToGroup(r *Result) {
	k := groupKey{t: r.Type, group: r.Group}
	group := dg.getGroup(k)

	r.GroupIndex = len(group.Results)
	group.Results = append(group.Results, r)
}

// String implements fmt.Stringer for Param.
func (p *Param) String() string {
	if p.Name != "" {
		return fmt.Sprintf("%v[name=%v]", p.Type.String(), p.Name)
	}
	return p.Type.String()
}

// String implements fmt.Stringer for Result.
func (r *Result) String() string {
	switch {
	case r.Name != "":
		return fmt.Sprintf("%v[name=%v]", r.Type.String(), r.Name)
	case r.Group != "":
		return fmt.Sprintf("%v[group=%v]%v", r.Type.String(), r.Group, r.GroupIndex)
	default:
		return r.Type.String()
	}
}

// String implements fmt.Stringer for Group.
func (g *Group) String() string {
	return fmt.Sprintf("[type=%v group=%v]", g.Type.String(), g.Group)
}

// Attributes composes and returns a string to style the Param's sublabel.
func (p *Param) Attributes() string {
	if p.Name != "" {
		return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Name: %v</FONT>`, p.Name)
	}
	return ""
}

// Attributes composes and returns a string to style the Result's sublabel.
func (r *Result) Attributes() string {
	switch {
	case r.Name != "":
		return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Name: %v</FONT>`, r.Name)
	case r.Group != "":
		return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Group: %v</FONT>`, r.Group)
	default:
		return ""
	}
}

// Attributes composes and returns a string to style the Group's sublabel.
func (g *Group) Attributes() string {
	return fmt.Sprintf(`<BR /><FONT POINT-SIZE="10">Group: %v</FONT>`, g.Group)
}

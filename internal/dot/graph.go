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

// FailureState of a constructor or group is updated when they fail to build.
type FailureState int

const (
	success FailureState = iota
	rootCause
	failed
)

// CtorID is a unique numeric identifier for constructors.
type CtorID uintptr

// Ctor encodes a constructor provided to the container for the DOT graph.
type Ctor struct {
	Name        string
	Package     string
	File        string
	Line        int
	ID          CtorID
	Params      []*Param
	GroupParams []*Group
	Results     []*Result
	State       FailureState
}

// Param is a parameter node in the graph.
type Param struct {
	*Node

	Optional bool
}

// Result is a result node in the graph.
type Result struct {
	*Node

	// GroupIndex is added to differentiate grouped values from one another.
	// Since grouped values have the same type and group, their Node / string
	// representations are the same so we need indices to uniquely identify
	// the values.
	GroupIndex int
}

// Group is a group node in the graph.
type Group struct {
	// Type is the type of values in the group.
	Type    reflect.Type
	Name    string
	Results []*Result
	State   FailureState
}

// Graph is the DOT-format graph in a Container.
type Graph struct {
	Ctors   []*Ctor
	ctorMap map[CtorID]*Ctor
	Groups  map[groupKey]*Group

	// RootCauses is a list of the point of failures. They are the root causes of
	// failed invokes and can be either missing types (not provided) or error
	// types (error providing).
	RootCauses []*Result

	// Failed is the list of nodes that failed to build due to missing/failed
	// dependencies.
	Failed []*Result
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
		ctorMap: make(map[CtorID]*Ctor),
		Groups:  make(map[groupKey]*Group),
	}
}

// NewGroup creates a new group with information in the groupKey.
func NewGroup(k groupKey) *Group {
	return &Group{
		Type: k.t,
		Name: k.group,
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
			dg.addToGroup(result, c.ID)
		}
	}

	c.Params = params
	c.GroupParams = groupParams
	c.Results = resultList

	dg.Ctors = append(dg.Ctors, c)
	dg.ctorMap[c.ID] = c
}

func (dg *Graph) failNode(r *Result, pof bool) {
	if pof {
		dg.RootCauses = append(dg.RootCauses, r)
	} else {
		dg.Failed = append(dg.Failed, r)
	}
}

// MissingNodes adds missing nodes to the list of failed Results in the graph.
func (dg *Graph) MissingNodes(results []*Result) {
	pof := len(dg.RootCauses) == 0

	for _, r := range results {
		dg.failNode(r, pof)
	}
}

// FailNodes adds results to the list of failed Results in the graph, and
// updates the state of the constructor with the given id accordingly.
func (dg *Graph) FailNodes(results []*Result, id CtorID) {
	pof := len(dg.RootCauses) == 0

	for _, r := range results {
		dg.failNode(r, pof)
	}

	if c, ok := dg.ctorMap[id]; ok {
		if pof {
			c.State = rootCause
		} else {
			c.State = failed
		}
	}
}

// FailGroupNodes finds and adds the failed grouped nodes to the list of failed
// Results in the graph, and updates the state of the group and constructor
// with the given id accordingly.
func (dg *Graph) FailGroupNodes(name string, t reflect.Type, id CtorID) {
	pof := len(dg.RootCauses) == 0

	k := groupKey{t: t, group: name}
	group := dg.getGroup(k)

	for _, r := range dg.ctorMap[id].Results {
		if r.Type == t && r.Group == name {
			dg.failNode(r, pof)
		}
	}

	if c, ok := dg.ctorMap[id]; ok {
		if pof {
			group.State = rootCause
			c.State = rootCause
		} else {
			group.State = failed
			c.State = failed
		}
	}
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
func (dg *Graph) addToGroup(r *Result, id CtorID) {
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
	return fmt.Sprintf("[type=%v group=%v]", g.Type.String(), g.Name)
}

// Attributes composes and returns a string of the Result node's attributes.
func (r *Result) Attributes() string {
	switch {
	case r.Name != "":
		return fmt.Sprintf(`label=<%v<BR /><FONT POINT-SIZE="10">Name: %v</FONT>>`, r.Type, r.Name)
	case r.Group != "":
		return fmt.Sprintf(`label=<%v<BR /><FONT POINT-SIZE="10">Group: %v</FONT>>`, r.Type, r.Group)
	default:
		return fmt.Sprintf(`label=<%v>`, r.Type)
	}
}

// Attributes composes and returns a string of the Group node's attributes.
func (g *Group) Attributes() string {
	attr := fmt.Sprintf(`shape=diamond label=<%v<BR /><FONT POINT-SIZE="10">Group: %v</FONT>>`, g.Type, g.Name)
	if g.State != success {
		attr += " color=" + g.State.Color()
	}
	return attr
}

// Color returns the color representation of each FailureState.
func (s FailureState) Color() string {
	switch s {
	case rootCause:
		return "red"
	case failed:
		return "orange"
	default:
		return "black"
	}
}

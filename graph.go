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
	// The index of this node in the graphHolder's nodes.
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
	nodes []*graphNode

	// Container whose graph this holder contains.
	c *Container

	// Used for snapshots and rollbacks.
	ss *graphSnapshot
}

func newGraphHolder(c *Container) *graphHolder {
	return &graphHolder{c: c}

}

func (gh *graphHolder) Order() int {
	return len(gh.nodes)
}

// EdgesFrom returns the indices of nodes that are dependencies of node u. To do that,
// it needs to do one of the following:
// 1. For a constructor node, iterate through its parameters and get the orders of its direct
// dependencies' providers.
// 2. For a value group node, look at the group providers and get their orders.
func (gh *graphHolder) EdgesFrom(u int) []int {
	n := gh.nodes[u]

	var orders []int

	switch w := n.Wrapped.(type) {
	case *constructorNode:
		for _, param := range w.paramList.Params {
			orders = append(orders, getParamOrder(gh, param)...)
		}
	case *paramGroupedSlice:
		providers := gh.c.getGroupProviders(w.Group, w.Type.Elem())
		for _, provider := range providers {
			orders = append(orders, provider.Order())
		}
	}
	return orders
}

// NewNode adds a new value to the graph and returns its order.
func (gh *graphHolder) NewNode(wrapped interface{}) int {
	order := len(gh.nodes)
	gh.nodes = append(gh.nodes, &graphNode{
		Order:   order,
		Wrapped: wrapped,
	})
	return order
}

// Lookup retrieves the value for the node with the given order.
// Lookup panics if i is invalid.
func (gh *graphHolder) Lookup(i int) interface{} {
	return gh.nodes[i].Wrapped
}

// graphSnapshot records a snapshotted state of a graph.
type graphSnapshot struct {
	nodesLength int
}

// Snapshot is a helper used for taking a temporary snapshot of the current state
// of the graph. Rollback() can be called subsequently to roll back the graph to
// the snapshotted state. Only one snapshot can exist per graph, so calling Snapshot
// many times overwrites the previous snapshotted state.
func (gh *graphHolder) Snapshot() {
	gh.ss = &graphSnapshot{
		nodesLength: len(gh.nodes),
	}
}

// Rollback is a method used for rolling back the state of the current graphHolder
// back to a snapshotted state, if one exists. It is a no-op if there is no snapshot.
func (gh *graphHolder) Rollback() {
	if gh.ss == nil {
		return
	}
	// nodes is an append-only list To rollback, we just drop the
	// extraneous entries from the slice.
	gh.nodes = gh.nodes[:gh.ss.nodesLength]
	gh.ss = nil
}

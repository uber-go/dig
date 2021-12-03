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

// Graph represents a simple interface for representation
// of a directed graph.
// It is assumed that each node in the graph is uniquely
// identified with an incremental positive integer (i.e. 1, 2, 3...).
// A value of 0 for a node represents a sentinel error value.
type Graph interface {
	// Order returns the total number of nodes in the graph
	Order() int

	// EdgesFrom returns a list of integers that each
	// represents a node that has an edge from node u.
	EdgesFrom(u int) []int
}

// IsAcyclic uses depth-first search to find cycles
// in a generic graph represented by Graph interface.
// If a cycle is found, it returns a list of nodes that
// are in the cyclic path, identified by their orders.
func IsAcyclic(g Graph) (bool, []int) {
	// cycleStart is a node that introduces a cycle in
	// the graph. Values in the range [0, g.Order()] mean
	// that there exists a cycle in g.
	cycleStart := -1
	info := newCycleInfo(g.Order())

	for i := 1; i < g.Order(); i++ {
		info.Reset()

		cycleStart = isAcyclic(g, i, info)
		if cycleStart >= 0 {
			break
		}
	}

	if cycleStart < 0 {
		return true, nil
	}

	// compute cycle path using backtrack
	cycle := []int{cycleStart}
	curr := cycleStart
	for {
		curr = info.nodes[curr].Backtrack
		cycle = append(cycle, curr)
		if curr == cycleStart {
			break
		}
	}

	// cycle is reverse-order.
	i, j := 0, len(cycle)-1
	for i < j {
		cycle[i], cycle[j] = cycle[j], cycle[i]
		i++
		j--
	}
	return false, cycle
}

// isAcyclic traverses the given graph starting from a specific node
// using depth-first search using recursion. If a cycle is detected,
// it returns the node that contains the "last" edge that introduces
// a cycle.
// For example, running isAcyclic starting from 1 on the following
// graph will return 3.
// 	1 -> 2 -> 3 -> 1
func isAcyclic(g Graph, u int, info *cycleInfo) int {
	info.nodes[u].Visited = true
	info.nodes[u].OnStack = true

	for _, v := range g.EdgesFrom(u) {
		if !info.nodes[v].Visited {
			info.nodes[v].Backtrack = u
			if start := isAcyclic(g, v, info); start >= 0 {
				return start
			}
		} else if info.nodes[v].OnStack {
			info.nodes[v].Backtrack = u
			return v
		}
	}
	info.nodes[u].OnStack = false
	return -1
}

// cycleNode keeps track of a single node's info for cycle detection.
type cycleNode struct {
	Visited   bool
	OnStack   bool
	Backtrack int
}

// cycleInfo contains helpful info for cycle detection.
type cycleInfo struct {
	// order is the number of nodes in the graph
	order int

	// nodes is the information for a given node.
	nodes []cycleNode
}

func newCycleInfo(order int) *cycleInfo {
	return &cycleInfo{
		order: order,
		// +1 because 0 is always a sentinel value.
		nodes: make([]cycleNode, order+1),
	}
}

func (info *cycleInfo) Reset() {
	for i := 1; i < info.order; i++ {
		info.nodes[i].Visited = false
		info.nodes[i].OnStack = false
		info.nodes[i].Backtrack = 0
	}
}
